package greeting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

type CronScheduler interface {
	RegisterCron(spec string, fn func()) (scheduler.JobID, error)
	Cancel(id scheduler.JobID)
}

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	sch       CronScheduler
	mu        sync.Mutex
	cronJobs  map[string][]scheduler.JobID
	now       func() time.Time
	voiceGate sharedtypes.ProactiveVoiceGate
}

func NewService(store *Store, xc xiaozhiclient.Client, schedulers ...CronScheduler) *Service {
	var sch CronScheduler
	if len(schedulers) > 0 {
		sch = schedulers[0]
	}
	return &Service{
		store:    store,
		xc:       xc,
		sch:      sch,
		cronJobs: make(map[string][]scheduler.JobID),
		now:      time.Now,
	}
}

func (s *Service) UseProactiveVoiceGate(gate sharedtypes.ProactiveVoiceGate) {
	s.voiceGate = gate
}

func (s *Service) TriggerProactiveGreeting(ctx context.Context, deviceID string) (sharedtypes.ProactiveGreetingResult, error) {
	greeting, err := s.Trigger(ctx, TriggerRequest{
		DeviceID:   deviceID,
		TonePreset: ToneCasual,
	})
	return sharedtypes.ProactiveGreetingResult{
		ID:           greeting.ID,
		Status:       string(greeting.Status),
		Text:         greeting.Text,
		ErrorMessage: greeting.ErrorMessage,
	}, err
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

	lease, err := s.tryAcquireProactiveVoice(ctx, greeting.DeviceID, now)
	if err != nil {
		greeting.Status = StatusSkipped
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, &greeting)
		return greeting, err
	}

	if err := s.xc.InjectSpeak(ctx, greeting.DeviceID, greeting.Text, xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		if lease != nil {
			_ = lease.Rollback(ctx)
		}
		greeting.Status = StatusFailed
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, &greeting)
		return greeting, err
	}
	if lease != nil {
		_ = lease.Commit(ctx)
	}

	playedAt := s.now().UTC()
	greeting.Status = StatusPlayed
	greeting.PlayedAt = &playedAt
	if err := s.store.Update(ctx, &greeting); err != nil {
		return Greeting{}, err
	}
	return greeting, nil
}

func (s *Service) tryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	if s.voiceGate == nil {
		return nil, nil
	}
	return s.voiceGate.TryAcquireProactiveVoice(ctx, deviceID, at)
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
	if _, err := s.registerSchedule(schedule); err != nil {
		return GreetingSchedule{}, err
	}
	return schedule, nil
}

func (s *Service) RestoreSchedules(ctx context.Context) (int, error) {
	schedules, err := s.store.ListSchedules(ctx)
	if err != nil {
		return 0, err
	}

	restored := 0
	for _, schedule := range schedules {
		count, err := s.registerSchedule(schedule)
		if err != nil {
			return restored, err
		}
		restored += count
	}
	return restored, nil
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

func (s *Service) registerSchedule(schedule GreetingSchedule) (int, error) {
	if s.sch == nil {
		return 0, nil
	}

	s.cancelScheduleJobs(schedule.DeviceID)
	count := 0
	for _, slot := range schedule.Slots {
		if !slot.Enabled {
			continue
		}
		spec, err := cronSpec(slot.Time)
		if err != nil {
			return count, err
		}
		deviceID := schedule.DeviceID
		slot := slot
		jobID, err := s.sch.RegisterCron(spec, func() {
			s.triggerScheduled(deviceID, slot)
		})
		if err != nil {
			return count, err
		}
		s.addScheduleJob(schedule.DeviceID, jobID)
		count++
	}
	return count, nil
}

func (s *Service) cancelScheduleJobs(deviceID string) {
	s.mu.Lock()
	jobs := append([]scheduler.JobID(nil), s.cronJobs[deviceID]...)
	delete(s.cronJobs, deviceID)
	s.mu.Unlock()

	for _, jobID := range jobs {
		s.sch.Cancel(jobID)
	}
}

func (s *Service) addScheduleJob(deviceID string, jobID scheduler.JobID) {
	s.mu.Lock()
	s.cronJobs[deviceID] = append(s.cronJobs[deviceID], jobID)
	s.mu.Unlock()
}

func (s *Service) triggerScheduled(deviceID string, slot ScheduleSlot) {
	_, _ = s.Trigger(context.Background(), TriggerRequest{
		DeviceID:   deviceID,
		TonePreset: slot.TonePreset,
	})
}

func cronSpec(slotTime string) (string, error) {
	parsed, err := time.Parse("15:04", slotTime)
	if err != nil {
		return "", ErrInvalidInput
	}
	return fmt.Sprintf("%d %d * * *", parsed.Minute(), parsed.Hour()), nil
}

func greetingText(tone TonePreset) string {
	if tone == ToneCasual {
		return "王阿姨，回来啦，今天过得怎么样？"
	}
	return "王阿姨，下午好~ 今天精神咋样？"
}
