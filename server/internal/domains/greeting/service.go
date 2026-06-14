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

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
}

const greetingRetryDelay = time.Minute

var requiredScheduleSlotLabels = map[string]struct{}{
	"morning": {},
	"noon":    {},
	"evening": {},
}

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	sch       CronScheduler
	retrySch  OneShotScheduler
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
	var retrySch OneShotScheduler
	if candidate, ok := sch.(OneShotScheduler); ok {
		retrySch = candidate
	}
	return &Service{
		store:    store,
		xc:       xc,
		sch:      sch,
		retrySch: retrySch,
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
	now := s.now().UTC()
	text := greetingText(now, tone)
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

	if err := s.play(ctx, &greeting, now); err != nil {
		return greeting, err
	}
	return greeting, nil
}

func (s *Service) play(ctx context.Context, greeting *Greeting, at time.Time) error {
	lease, err := s.tryAcquireProactiveVoice(ctx, greeting.DeviceID, at)
	if err != nil {
		if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
			if queueErr := s.queueRetry(ctx, greeting, at, err); queueErr == nil {
				return nil
			}
		}
		greeting.Status = StatusSkipped
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, greeting)
		return err
	}

	if err := s.xc.InjectSpeak(ctx, greeting.DeviceID, greeting.Text, proactiveSpeakOptions()); err != nil {
		if lease != nil {
			_ = lease.Rollback(ctx)
		}
		greeting.Status = StatusFailed
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, greeting)
		return err
	}
	if lease != nil {
		_ = lease.Commit(ctx)
	}

	playedAt := s.now().UTC()
	greeting.Status = StatusPlayed
	greeting.PlayedAt = &playedAt
	greeting.ErrorMessage = ""
	if err := s.store.Update(ctx, greeting); err != nil {
		return err
	}
	return nil
}

func (s *Service) queueRetry(ctx context.Context, greeting *Greeting, at time.Time, cause error) error {
	if s.retrySch == nil {
		return cause
	}

	retryAt := at.UTC().Add(greetingRetryDelay)
	if _, err := s.retrySch.ScheduleAt(retryAt, func() {
		s.retryQueuedGreeting(greeting.ID)
	}); err != nil {
		greeting.Status = StatusFailed
		greeting.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, greeting)
		return err
	}

	greeting.Status = StatusPending
	greeting.ErrorMessage = cause.Error()
	return s.store.Update(ctx, greeting)
}

func (s *Service) retryQueuedGreeting(id uint) {
	ctx := context.Background()
	greeting, err := s.store.Get(ctx, id)
	if err != nil || greeting.Status != StatusPending {
		return
	}
	_ = s.play(ctx, &greeting, s.now().UTC())
}

func (s *Service) tryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	if s.voiceGate == nil {
		return nil, nil
	}
	return s.voiceGate.TryAcquireProactiveVoice(ctx, deviceID, at)
}

func proactiveSpeakOptions() xiaozhiclient.InjectOptions {
	autoListen := true
	return xiaozhiclient.InjectOptions{SkipLLM: true, AutoListen: &autoListen}
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Greeting, error) {
	filter.DeviceID = strings.TrimSpace(filter.DeviceID)
	filter.Status = Status(strings.TrimSpace(string(filter.Status)))
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
	seenLabels := make(map[string]struct{}, len(req.Slots))
	for _, slot := range req.Slots {
		normalized, err := normalizeScheduleSlot(slot)
		if err != nil {
			return GreetingSchedule{}, err
		}
		if _, exists := seenLabels[normalized.Label]; exists {
			return GreetingSchedule{}, ErrInvalidInput
		}
		seenLabels[normalized.Label] = struct{}{}
		slots = append(slots, normalized)
	}
	if !hasRequiredScheduleSlotLabels(seenLabels) {
		return GreetingSchedule{}, ErrInvalidInput
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
	if !isRequiredScheduleSlotLabel(label) {
		return ScheduleSlot{}, ErrInvalidInput
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

func isRequiredScheduleSlotLabel(label string) bool {
	_, ok := requiredScheduleSlotLabels[label]
	return ok
}

func hasRequiredScheduleSlotLabels(seenLabels map[string]struct{}) bool {
	if len(seenLabels) != len(requiredScheduleSlotLabels) {
		return false
	}
	for label := range requiredScheduleSlotLabels {
		if _, ok := seenLabels[label]; !ok {
			return false
		}
	}
	return true
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

// greetingLocation 让时段问候按东八区判断，避免容器 UTC 下早晨也说"下午好"。
var greetingLocation = time.FixedZone("CST", 8*60*60)

func greetingText(now time.Time, tone TonePreset) string {
	if tone == ToneCasual {
		return "您回来啦，今天过得怎么样？"
	}
	return timeOfDayGreeting(now) + "您今天精神怎么样？"
}

func timeOfDayGreeting(now time.Time) string {
	switch h := now.In(greetingLocation).Hour(); {
	case h < 11:
		return "早上好~ "
	case h < 13:
		return "中午好~ "
	case h < 18:
		return "下午好~ "
	default:
		return "晚上好~ "
	}
}
