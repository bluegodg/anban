package reminder

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
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
	list, err := s.store.List(ctx, ListFilter{})
	if err != nil {
		return
	}
	for _, rem := range list {
		if rem.ID != id {
			continue
		}
		s.play(ctx, &rem)
		return
	}
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
	_ = s.store.Update(ctx, rem)
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
