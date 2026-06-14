package reminder

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const (
	defaultAckTimeout    = 30 * time.Minute
	proactiveRetryDelay  = time.Minute
	minReminderTextRunes = 30
	maxReminderTextRunes = 60
)

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
	Cancel(id scheduler.JobID)
}

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	sch       OneShotScheduler
	now       func() time.Time
	voiceGate sharedtypes.ProactiveVoiceGate
}

func NewService(store *Store, xc xiaozhiclient.Client, sch OneShotScheduler) *Service {
	return &Service{
		store: store,
		xc:    xc,
		sch:   sch,
		now:   time.Now,
	}
}

func (s *Service) UseProactiveVoiceGate(gate sharedtypes.ProactiveVoiceGate) {
	s.voiceGate = gate
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Reminder, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	content := strings.TrimSpace(req.Content)
	scheduledAt := req.ScheduledAt.UTC()
	if deviceID == "" || content == "" || scheduledAt.IsZero() || !scheduledAt.After(s.now().UTC()) {
		return Reminder{}, ErrInvalidInput
	}

	category := normalizeCategory(req.Category)
	rem := Reminder{
		DeviceID:    deviceID,
		ScheduledAt: scheduledAt,
		Content:     content,
		Category:    category,
		Text:        reminderText(content, category),
		Status:      StatusScheduled,
	}
	if err := s.store.Create(ctx, &rem); err != nil {
		return Reminder{}, err
	}

	jobID, err := s.sch.ScheduleAt(rem.ScheduledAt, func() {
		s.fire(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, &rem)
		return rem, err
	}
	rem.JobID = string(jobID)
	if err := s.store.Update(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Reminder, error) {
	filter.DeviceID = strings.TrimSpace(filter.DeviceID)
	filter.Status = Status(strings.TrimSpace(string(filter.Status)))
	return s.store.List(ctx, filter)
}

func (s *Service) Cancel(ctx context.Context, id uint) (Reminder, error) {
	if id == 0 {
		return Reminder{}, ErrInvalidInput
	}
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return Reminder{}, err
	}
	if rem.Status != StatusScheduled {
		return rem, nil
	}
	if rem.JobID != "" {
		s.sch.Cancel(scheduler.JobID(rem.JobID))
	}
	rem.Status = StatusCanceled
	rem.JobID = ""
	if err := s.store.Update(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func (s *Service) Acknowledge(ctx context.Context, id uint, req AckRequest) (Reminder, error) {
	if id == 0 {
		return Reminder{}, ErrInvalidInput
	}
	return s.acknowledge(ctx, id, normalizeAckKind(req.AckKind), true)
}

func (s *Service) RestoreScheduled(ctx context.Context) (int, error) {
	reminders, err := s.store.List(ctx, ListFilter{Status: StatusScheduled})
	if err != nil {
		return 0, err
	}

	restored := 0
	for i := range reminders {
		rem := reminders[i]
		jobID, err := s.sch.ScheduleAt(rem.ScheduledAt, func() {
			s.fire(rem.ID)
		})
		if err != nil {
			rem.Status = StatusFailed
			rem.ErrorMessage = err.Error()
			_ = s.store.Update(ctx, &rem)
			return restored, err
		}
		rem.JobID = string(jobID)
		if err := s.store.Update(ctx, &rem); err != nil {
			return restored, err
		}
		restored++
	}

	played, err := s.store.List(ctx, ListFilter{Status: StatusPlayed})
	if err != nil {
		return restored, err
	}
	for i := range played {
		rem := played[i]
		if rem.PlayedAt == nil {
			continue
		}
		timeoutAt := rem.PlayedAt.UTC().Add(defaultAckTimeout)
		if !timeoutAt.After(s.now().UTC()) {
			if _, err := s.acknowledge(ctx, rem.ID, AckKindTimeout, false); err != nil {
				return restored, err
			}
			restored++
			continue
		}
		jobID, err := s.sch.ScheduleAt(timeoutAt, func() {
			s.markUnanswered(rem.ID)
		})
		if err != nil {
			rem.Status = StatusFailed
			rem.ErrorMessage = err.Error()
			_ = s.store.Update(ctx, &rem)
			return restored, err
		}
		rem.AckJobID = string(jobID)
		if err := s.store.Update(ctx, &rem); err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

func (s *Service) fire(id uint) {
	ctx := context.Background()
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return
	}
	if rem.Status != StatusScheduled {
		return
	}
	s.play(ctx, &rem)
}

func (s *Service) play(ctx context.Context, rem *Reminder) {
	playedAt := s.now().UTC()
	lease, err := s.tryAcquireProactiveVoice(ctx, rem.DeviceID, playedAt)
	if err != nil {
		if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
			s.requeueProactiveVoice(ctx, rem, playedAt, err)
			return
		}
		rem.Status = StatusSkipped
		rem.ErrorMessage = err.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	if err := s.xc.InjectSpeak(ctx, rem.DeviceID, rem.Text, proactiveSpeakOptions()); err != nil {
		if lease != nil {
			_ = lease.Rollback(ctx)
		}
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, rem)
		return
	}
	if lease != nil {
		_ = lease.Commit(ctx)
	}

	rem.Status = StatusPlayed
	rem.PlayedAt = &playedAt
	rem.JobID = ""
	jobID, err := s.sch.ScheduleAt(playedAt.Add(defaultAckTimeout), func() {
		s.markUnanswered(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, rem)
		return
	}
	rem.AckJobID = string(jobID)
	_ = s.store.Update(ctx, rem)
}

func (s *Service) requeueProactiveVoice(ctx context.Context, rem *Reminder, at time.Time, cause error) {
	retryAt := at.UTC().Add(proactiveRetryDelay)
	if s.sch == nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = cause.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	jobID, err := s.sch.ScheduleAt(retryAt, func() {
		s.fire(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	rem.Status = StatusScheduled
	rem.ScheduledAt = retryAt
	rem.JobID = string(jobID)
	rem.ErrorMessage = cause.Error()
	_ = s.store.Update(ctx, rem)
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

func (s *Service) markUnanswered(id uint) {
	_, _ = s.acknowledge(context.Background(), id, AckKindTimeout, false)
}

func (s *Service) acknowledge(ctx context.Context, id uint, kind AckKind, cancelJob bool) (Reminder, error) {
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return Reminder{}, err
	}

	switch rem.Status {
	case StatusPlayed:
	case StatusCompleted, StatusUnanswered:
		return rem, nil
	default:
		return Reminder{}, ErrInvalidInput
	}

	if cancelJob && rem.AckJobID != "" {
		s.sch.Cancel(scheduler.JobID(rem.AckJobID))
	}

	ackAt := s.now().UTC()
	if kind == AckKindTimeout {
		rem.Status = StatusUnanswered
	} else {
		rem.Status = StatusCompleted
	}
	rem.AckKind = kind
	rem.AcknowledgedAt = &ackAt
	rem.AckJobID = ""
	if err := s.store.Update(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func normalizeAckKind(kind AckKind) AckKind {
	if kind == AckKindTimeout {
		return AckKindTimeout
	}
	return AckKindVoice
}

func normalizeCategory(category Category) Category {
	switch category {
	case CategoryMed, CategoryBirthday, CategoryFestival:
		return category
	default:
		return CategoryCustom
	}
}

func reminderText(content string, category Category) string {
	content = strings.TrimSpace(content)
	if category == CategoryMed {
		return buildReminderText(
			"王阿姨，该",
			content,
			"啦。小宝惦记着您，记得按时完成，安心一点哦。",
		)
	}
	return buildReminderText(
		"王阿姨，提醒您：",
		content,
		"。完成后跟安伴说一声好，我也放心。",
	)
}

func buildReminderText(prefix, content, suffix string) string {
	maxContentRunes := maxReminderTextRunes - runeLen(prefix) - runeLen(suffix)
	if maxContentRunes < 0 {
		return truncateRunes(prefix+suffix, maxReminderTextRunes)
	}

	text := prefix + truncateRunes(content, maxContentRunes) + suffix
	if runeLen(text) < minReminderTextRunes {
		text += "安伴会陪着您。"
	}
	return truncateRunes(text, maxReminderTextRunes)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func runeLen(value string) int {
	return len([]rune(value))
}
