package reminder

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

const defaultAckTimeout = 30 * time.Minute

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
	Cancel(id scheduler.JobID)
}

type Service struct {
	store *Store
	xc    xiaozhiclient.Client
	sch   OneShotScheduler
	now   func() time.Time
}

func NewService(store *Store, xc xiaozhiclient.Client, sch OneShotScheduler) *Service {
	return &Service{
		store: store,
		xc:    xc,
		sch:   sch,
		now:   time.Now,
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Reminder, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	content := strings.TrimSpace(req.Content)
	if deviceID == "" || content == "" || req.ScheduledAt.IsZero() {
		return Reminder{}, ErrInvalidInput
	}

	category := normalizeCategory(req.Category)
	rem := Reminder{
		DeviceID:    deviceID,
		ScheduledAt: req.ScheduledAt.UTC(),
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
	if err := s.xc.InjectSpeak(ctx, rem.DeviceID, rem.Text, xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, rem)
		return
	}

	playedAt := s.now().UTC()
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
	if category == CategoryMed {
		return "王阿姨，该" + content + "啦~ 小宝昨天还问起您有没有按时测呢。"
	}
	return "王阿姨，提醒您：" + content + "。记得照顾好自己呀。"
}
