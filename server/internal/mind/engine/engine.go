package engine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/behavior"
	"github.com/bluegodg/anban/server/internal/mind/drives"
	"github.com/bluegodg/anban/server/internal/mind/expression"
	"github.com/bluegodg/anban/server/internal/mind/life"
	"github.com/bluegodg/anban/server/internal/mind/reflection"
	"github.com/bluegodg/anban/server/internal/mind/selfstate"
	"github.com/bluegodg/anban/server/internal/mind/situation"
	"github.com/bluegodg/anban/server/internal/mind/thoughts"
)

type Service struct {
	store    *mind.Store
	executor ActionExecutor
	location *time.Location
}

type ExecutionResult struct {
	Status       mind.ActionStatus
	ExecutorRef  string
	ErrorMessage string
}

type ActionExecutor interface {
	Execute(ctx context.Context, action mind.Action) (ExecutionResult, error)
}

func New(store *mind.Store) *Service {
	return &Service{store: store}
}

func (s *Service) UseExecutor(executor ActionExecutor) {
	s.executor = executor
}

func (s *Service) UseLocation(location *time.Location) {
	s.location = location
}

func (s *Service) Ingest(ctx context.Context, event mind.Event) ([]mind.Action, error) {
	event.DeviceID = strings.TrimSpace(event.DeviceID)
	if event.DeviceID == "" {
		return nil, mind.ErrInvalidInput
	}
	event.ID = strings.TrimSpace(event.ID)
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	if event.ID == "" {
		generatedID, err := generateEventID(event)
		if err != nil {
			return nil, err
		}
		event.ID = generatedID
	}

	var actions []mind.Action
	var duplicate bool
	err := s.store.WithinTransaction(ctx, func(txStore *mind.Store) error {
		if err := txStore.AppendEvent(ctx, event); err != nil {
			if errors.Is(err, mind.ErrDuplicateEvent) {
				duplicate = true
				return nil
			}
			return err
		}

		lateEvent, err := txStore.HasEventAfter(ctx, event.DeviceID, event.At)
		if err != nil {
			return err
		}
		recent, err := txStore.ListRecentEventsAtOrBefore(ctx, event.DeviceID, event.At, 20)
		if err != nil {
			return err
		}

		actions, err = s.runPipeline(ctx, txStore, event.DeviceID, event.At, event, recent, lateEvent)
		return err
	})
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, nil
	}
	if err := s.executePendingActions(ctx, actions); err != nil {
		return actions, err
	}
	return actions, nil
}

func generateEventID(event mind.Event) (string, error) {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return "", err
	}
	seed := fmt.Sprintf("%s|%s|%s|%d|%s|%s", event.DeviceID, event.Source, event.Type, event.At.UnixNano(), event.Summary, payload)
	sum := sha256.Sum256([]byte(seed))
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("evt-%x-%s", sum[:12], hex.EncodeToString(suffix[:])), nil
}

func (s *Service) TickIdle(ctx context.Context, deviceID string, at time.Time) ([]mind.Action, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, mind.ErrInvalidInput
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return s.Ingest(ctx, mind.Event{
		ID:         fmt.Sprintf("evt-%s-idle-%d", deviceID, at.UnixNano()),
		DeviceID:   deviceID,
		Type:       mind.EventLongSilence,
		Source:     mind.SourceMind,
		At:         at,
		Summary:    "空闲循环检测到一段沉默",
		Salience:   0.45,
		Emotion:    "quiet",
		Confidence: 0.7,
	})
}

func (s *Service) Reflect(ctx context.Context, deviceID string, window mind.TimeWindow) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || window.From.IsZero() || window.To.IsZero() || window.From.After(window.To) {
		return mind.ErrInvalidInput
	}
	feedback, err := s.store.ListFeedback(ctx, deviceID, window)
	if err != nil {
		return err
	}
	ref := reflection.Summarize(deviceID, window.To, feedback)
	ref.ID = fmt.Sprintf("reflection-%s-%d-%d", deviceID, window.From.UnixNano(), window.To.UnixNano())

	return s.store.WithinTransaction(ctx, func(txStore *mind.Store) error {
		exists, err := txStore.ReflectionExists(ctx, ref.ID)
		if err != nil {
			return err
		}
		if !exists && len(ref.StateAdjustments) > 0 {
			state, err := txStore.GetSelfState(ctx, deviceID)
			if errors.Is(err, mind.ErrNotFound) {
				state = selfstate.Default(deviceID, window.To)
			} else if err != nil {
				return err
			}
			state = applyReflectionAdjustments(state, window.To, ref.StateAdjustments)
			if err := txStore.SaveSelfState(ctx, state); err != nil {
				return err
			}
		}
		return txStore.SaveReflection(ctx, ref)
	})
}

func (s *Service) UpdateLife(ctx context.Context, deviceID string, at time.Time) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return mind.ErrInvalidInput
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	recent, err := s.store.ListRecentEventsAtOrBefore(ctx, deviceID, at, 50)
	if err != nil {
		return err
	}
	state, err := s.store.GetSelfState(ctx, deviceID)
	if errors.Is(err, mind.ErrNotFound) {
		state = selfstate.Default(deviceID, at)
	} else if err != nil {
		return err
	}

	return s.store.SaveLifeState(ctx, life.Update(deviceID, at, state, recent))
}

func (s *Service) runPipeline(ctx context.Context, store *mind.Store, deviceID string, at time.Time, currentEvent mind.Event, recent []mind.Event, lateEvent bool) ([]mind.Action, error) {
	sit := situation.BuildWithLocation(deviceID, at, recent, s.location)

	var state mind.SelfState
	if lateEvent {
		state = selfstate.Default(deviceID, at)
	} else {
		var err error
		state, err = store.GetSelfState(ctx, deviceID)
		if errors.Is(err, mind.ErrNotFound) {
			state = selfstate.Default(deviceID, at)
		} else if err != nil {
			return nil, err
		}
	}
	state = selfstate.ApplyEvents(state, recent)
	if !lateEvent {
		if err := store.SaveSelfState(ctx, state); err != nil {
			return nil, err
		}
	}

	activeDrives := drives.Activate(sit, state, recent)
	// recent is already filtered to events at or before the current event time;
	// thought/action generation stays anchored to the just-appended event so late
	// arrivals do not replay an already-processed newer event.
	generatedThoughts := thoughts.Generate(sit, state, activeDrives, []mind.Event{currentEvent})
	for _, thought := range generatedThoughts {
		if err := store.SaveThought(ctx, thought); err != nil {
			return nil, err
		}
	}

	candidates := behavior.Select(sit, state, generatedThoughts)
	out := make([]mind.Action, 0, len(candidates))
	for _, candidate := range candidates {
		selected := expression.Gate(candidate, sit, state)
		selected.Args = mergeActionArgs(selected.Args, currentEvent.Payload)
		if err := store.SaveAction(ctx, selected); err != nil {
			return nil, err
		}
		out = append(out, selected)
	}
	return out, nil
}

func (s *Service) executePendingActions(ctx context.Context, actions []mind.Action) error {
	if s.executor == nil {
		return nil
	}

	for i := range actions {
		if actions[i].Status != mind.ActionPending {
			continue
		}

		result, err := s.executor.Execute(ctx, actions[i])
		updated := actions[i]
		switch {
		case result.Status != "":
			updated.Status = result.Status
		case err != nil:
			updated.Status = mind.ActionFailed
		default:
			updated.Status = mind.ActionExecuted
		}
		updated.ExecutorRef = result.ExecutorRef
		if result.ErrorMessage != "" {
			updated.Reason = result.ErrorMessage
		} else if err != nil {
			updated.Reason = err.Error()
		}

		if saveErr := s.saveActionExecutionResult(ctx, updated); saveErr != nil {
			return saveErr
		}
		actions[i] = updated
		if err != nil && !isSafeSkippedStatus(updated.Status) {
			return err
		}
	}
	return nil
}

func isSafeSkippedStatus(status mind.ActionStatus) bool {
	return status == mind.ActionDeferred || status == mind.ActionSuppressed
}

func (s *Service) saveActionExecutionResult(ctx context.Context, action mind.Action) error {
	return s.store.WithinTransaction(ctx, func(txStore *mind.Store) error {
		if err := txStore.SaveAction(ctx, action); err != nil {
			return err
		}
		err := txStore.AppendEvent(ctx, actionExecutionEvent(action))
		if errors.Is(err, mind.ErrDuplicateEvent) {
			return nil
		}
		return err
	})
}

func actionExecutionEvent(action mind.Action) mind.Event {
	return mind.Event{
		ID:       fmt.Sprintf("evt-action-%s-result", action.ID),
		DeviceID: action.DeviceID,
		Type:     mind.EventActionExecuted,
		Source:   mind.SourceMind,
		At:       time.Now().UTC(),
		Summary:  "心智动作执行结果已记录",
		Payload: map[string]any{
			"actionId":    action.ID,
			"actionType":  string(action.Type),
			"executor":    action.Executor,
			"status":      string(action.Status),
			"executorRef": action.ExecutorRef,
			"reason":      action.Reason,
		},
		Salience:   actionExecutionSalience(action.Status),
		Emotion:    actionExecutionEmotion(action.Status),
		Confidence: 1,
	}
}

func actionExecutionSalience(status mind.ActionStatus) float64 {
	switch status {
	case mind.ActionFailed:
		return 0.75
	case mind.ActionDeferred:
		return 0.45
	case mind.ActionExecuted:
		return 0.6
	default:
		return 0.35
	}
}

func actionExecutionEmotion(status mind.ActionStatus) string {
	switch status {
	case mind.ActionFailed:
		return "concerned"
	case mind.ActionDeferred:
		return "patient"
	case mind.ActionExecuted:
		return "settled"
	default:
		return "neutral"
	}
}

func applyReflectionAdjustments(state mind.SelfState, at time.Time, adjustments map[string]float64) mind.SelfState {
	state.At = at
	for name, delta := range adjustments {
		switch name {
		case "warmth":
			state.Warmth = clamp01(state.Warmth + delta)
		case "concern":
			state.Concern = clamp01(state.Concern + delta)
		case "curiosity":
			state.Curiosity = clamp01(state.Curiosity + delta)
		case "playfulness":
			state.Playfulness = clamp01(state.Playfulness + delta)
		case "energy":
			state.Energy = clamp01(state.Energy + delta)
		case "quietness":
			state.Quietness = clamp01(state.Quietness + delta)
		case "patience":
			state.Patience = clamp01(state.Patience + delta)
		case "confidence":
			state.Confidence = clamp01(state.Confidence + delta)
		case "family_weight":
			state.FamilyWeight = clamp01(state.FamilyWeight + delta)
		case "pet_weight":
			state.PetWeight = clamp01(state.PetWeight + delta)
		case "steward_weight":
			state.StewardWeight = clamp01(state.StewardWeight + delta)
		}
	}
	return state
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func mergeActionArgs(actionArgs map[string]any, eventPayload map[string]any) map[string]any {
	if len(eventPayload) == 0 {
		return actionArgs
	}
	merged := make(map[string]any, len(eventPayload)+len(actionArgs))
	for key, value := range eventPayload {
		merged[key] = value
	}
	for key, value := range actionArgs {
		merged[key] = value
	}
	return merged
}
