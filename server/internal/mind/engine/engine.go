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
	"github.com/bluegodg/anban/server/internal/mind/selfstate"
	"github.com/bluegodg/anban/server/internal/mind/situation"
	"github.com/bluegodg/anban/server/internal/mind/thoughts"
)

type Service struct {
	store    *mind.Store
	executor ActionExecutor
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
	reflection := mind.Reflection{
		ID:               fmt.Sprintf("reflection-%s-%d-%d", deviceID, window.From.UnixNano(), window.To.UnixNano()),
		DeviceID:         deviceID,
		At:               window.To,
		EpisodeSummary:   "本轮反思已记录，具体摘要由 reflection 模块补充",
		StateAdjustments: map[string]float64{},
		BehaviorLessons:  []string{},
	}
	return s.store.SaveReflection(ctx, reflection)
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
	sit := situation.Build(deviceID, at, recent)

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

		if saveErr := s.store.SaveAction(ctx, updated); saveErr != nil {
			return saveErr
		}
		actions[i] = updated
		if err != nil {
			return err
		}
	}
	return nil
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
