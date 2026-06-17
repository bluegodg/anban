package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/behavior"
	"github.com/bluegodg/anban/server/internal/mind/drives"
	"github.com/bluegodg/anban/server/internal/mind/expression"
	"github.com/bluegodg/anban/server/internal/mind/selfstate"
	"github.com/bluegodg/anban/server/internal/mind/situation"
	"github.com/bluegodg/anban/server/internal/mind/thoughts"
)

type Service struct {
	store *mind.Store
}

func New(store *mind.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Ingest(ctx context.Context, event mind.Event) ([]mind.Action, error) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%s-%d", event.DeviceID, event.At.UnixNano())
	}
	if err := s.store.AppendEvent(ctx, event); err != nil {
		return nil, err
	}

	recent, err := s.store.ListRecentEvents(ctx, event.DeviceID, 20)
	if err != nil {
		return nil, err
	}
	return s.runPipeline(ctx, event.DeviceID, event.At, recent)
}

func (s *Service) TickIdle(ctx context.Context, deviceID string, at time.Time) ([]mind.Action, error) {
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
	reflection := mind.Reflection{
		ID:               fmt.Sprintf("reflection-%s-%d", deviceID, window.To.UnixNano()),
		DeviceID:         deviceID,
		At:               window.To,
		EpisodeSummary:   "本轮反思已记录，具体摘要由 reflection 模块补充",
		StateAdjustments: map[string]float64{},
		BehaviorLessons:  []string{},
	}
	return s.store.SaveReflection(ctx, reflection)
}

func (s *Service) runPipeline(ctx context.Context, deviceID string, at time.Time, recent []mind.Event) ([]mind.Action, error) {
	sit := situation.Build(deviceID, at, recent)

	state, err := s.store.GetSelfState(ctx, deviceID)
	if errors.Is(err, mind.ErrNotFound) {
		state = selfstate.Default(deviceID, at)
	} else if err != nil {
		return nil, err
	}
	state = selfstate.ApplyEvents(state, recent)
	if err := s.store.SaveSelfState(ctx, state); err != nil {
		return nil, err
	}

	activeDrives := drives.Activate(sit, state, recent)
	generatedThoughts := thoughts.Generate(sit, state, activeDrives, newestEvent(recent))
	for _, thought := range generatedThoughts {
		if err := s.store.SaveThought(ctx, thought); err != nil {
			return nil, err
		}
	}

	candidates := behavior.Select(sit, state, generatedThoughts)
	out := make([]mind.Action, 0, len(candidates))
	for _, candidate := range candidates {
		selected := expression.Gate(candidate, sit, state)
		if err := s.store.SaveAction(ctx, selected); err != nil {
			return nil, err
		}
		out = append(out, selected)
	}
	return out, nil
}

func newestEvent(recent []mind.Event) []mind.Event {
	if len(recent) == 0 {
		return nil
	}
	return recent[:1]
}
