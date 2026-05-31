package greeting

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Service struct {
	store *Store
	xc    xiaozhiclient.Client
	now   func() time.Time
}

func NewService(store *Store, xc xiaozhiclient.Client) *Service {
	return &Service{
		store: store,
		xc:    xc,
		now:   time.Now,
	}
}

func (s *Service) Trigger(ctx context.Context, req TriggerRequest) (Greeting, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Greeting{}, ErrInvalidInput
	}

	tone := normalizeTone(req.TonePreset)
	text := greetingText(tone)
	now := s.now().UTC()
	greeting := Greeting{
		DeviceID:    deviceID,
		TonePreset:  tone,
		Text:        text,
		Status:      StatusPending,
		TriggeredAt: now,
	}
	if err := s.store.Create(ctx, &greeting); err != nil {
		return Greeting{}, err
	}

	if err := s.xc.InjectSpeak(ctx, greeting.DeviceID, greeting.Text, xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		greeting.Status = StatusFailed
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, &greeting)
		return greeting, err
	}

	playedAt := s.now().UTC()
	greeting.Status = StatusPlayed
	greeting.PlayedAt = &playedAt
	if err := s.store.Update(ctx, &greeting); err != nil {
		return Greeting{}, err
	}
	return greeting, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Greeting, error) {
	return s.store.List(ctx, filter)
}

func normalizeTone(tone TonePreset) TonePreset {
	if tone == ToneCasual {
		return ToneCasual
	}
	return ToneWarm
}

func greetingText(tone TonePreset) string {
	if tone == ToneCasual {
		return "王阿姨，回来啦，今天过得怎么样？"
	}
	return "王阿姨，下午好~ 今天精神咋样？"
}
