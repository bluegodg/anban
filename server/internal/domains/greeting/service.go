package greeting

import (
	"context"
	"errors"
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

func (s *Service) GetSchedule(ctx context.Context, deviceID string) (GreetingSchedule, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return GreetingSchedule{}, ErrInvalidInput
	}

	schedule, err := s.store.GetSchedule(ctx, deviceID)
	if errors.Is(err, ErrNotFound) {
		return defaultSchedule(deviceID), nil
	}
	if err != nil {
		return GreetingSchedule{}, err
	}
	return schedule, nil
}

func (s *Service) UpdateSchedule(ctx context.Context, req ScheduleRequest) (GreetingSchedule, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" || len(req.Slots) == 0 {
		return GreetingSchedule{}, ErrInvalidInput
	}

	slots := make([]ScheduleSlot, 0, len(req.Slots))
	for _, slot := range req.Slots {
		normalized, err := normalizeScheduleSlot(slot)
		if err != nil {
			return GreetingSchedule{}, err
		}
		slots = append(slots, normalized)
	}

	schedule := GreetingSchedule{
		DeviceID: deviceID,
		Slots:    slots,
	}
	if err := s.store.UpsertSchedule(ctx, &schedule); err != nil {
		return GreetingSchedule{}, err
	}
	return schedule, nil
}

func normalizeTone(tone TonePreset) TonePreset {
	if tone == ToneCasual {
		return ToneCasual
	}
	return ToneWarm
}

func defaultSchedule(deviceID string) GreetingSchedule {
	return GreetingSchedule{
		DeviceID: deviceID,
		Slots: []ScheduleSlot{
			{Label: "morning", Time: "08:00", Enabled: true, TonePreset: ToneWarm},
			{Label: "noon", Time: "12:30", Enabled: true, TonePreset: ToneWarm},
			{Label: "evening", Time: "18:00", Enabled: true, TonePreset: ToneWarm},
		},
	}
}

func normalizeScheduleSlot(slot ScheduleSlot) (ScheduleSlot, error) {
	label := strings.TrimSpace(slot.Label)
	if label == "" {
		label = "custom"
	}
	slotTime := strings.TrimSpace(slot.Time)
	if _, err := time.Parse("15:04", slotTime); err != nil {
		return ScheduleSlot{}, ErrInvalidInput
	}

	return ScheduleSlot{
		Label:      label,
		Time:       slotTime,
		Enabled:    slot.Enabled,
		TonePreset: normalizeTone(slot.TonePreset),
	}, nil
}

func greetingText(tone TonePreset) string {
	if tone == ToneCasual {
		return "王阿姨，回来啦，今天过得怎么样？"
	}
	return "王阿姨，下午好~ 今天精神咋样？"
}
