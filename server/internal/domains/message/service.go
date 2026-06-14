package message

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const messageRetryDelay = time.Minute

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
}

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	retrySch  OneShotScheduler
	now       func() time.Time
	voiceGate sharedtypes.ProactiveVoiceGate
}

func NewService(store *Store, xc xiaozhiclient.Client, schedulers ...OneShotScheduler) *Service {
	var retrySch OneShotScheduler
	if len(schedulers) > 0 {
		retrySch = schedulers[0]
	}
	return &Service{
		store:    store,
		xc:       xc,
		retrySch: retrySch,
		now:      time.Now,
	}
}

func (s *Service) UseProactiveVoiceGate(gate sharedtypes.ProactiveVoiceGate) {
	s.voiceGate = gate
}

func (s *Service) Send(ctx context.Context, req SendRequest) (Message, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	text := trimAndLimit(req.Text, MaxTextRunes)
	if deviceID == "" || text == "" {
		return Message{}, ErrInvalidInput
	}

	now := s.now().UTC()
	msg := Message{
		DeviceID: deviceID,
		Text:     text,
		FromName: strings.TrimSpace(req.FromName),
		Status:   StatusPending,
		QueuedAt: now,
	}
	if err := s.store.Create(ctx, &msg); err != nil {
		return Message{}, err
	}

	if err := s.play(ctx, &msg, now); err != nil {
		return msg, err
	}
	return msg, nil
}

func (s *Service) play(ctx context.Context, msg *Message, at time.Time) error {
	speakText := msg.Text
	if msg.FromName != "" {
		speakText = "刚才" + msg.FromName + "发来留言：" + msg.Text
	}

	lease, err := s.tryAcquireProactiveVoice(ctx, msg.DeviceID, at)
	if err != nil {
		if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
			if queueErr := s.queueRetry(ctx, msg, at, err); queueErr == nil {
				return nil
			}
		}
		msg.Status = StatusFailed
		msg.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, msg)
		return err
	}

	if err := s.xc.InjectSpeak(ctx, msg.DeviceID, speakText, messageSpeakOptions()); err != nil {
		if lease != nil {
			_ = lease.Rollback(ctx)
		}
		msg.Status = StatusFailed
		msg.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, msg)
		return err
	}
	if lease != nil {
		_ = lease.Commit(ctx)
	}

	playedAt := s.now().UTC()
	msg.Status = StatusPlayed
	msg.PlayedAt = &playedAt
	msg.ErrorMessage = ""
	if err := s.store.Update(ctx, msg); err != nil {
		return err
	}
	return nil
}

func (s *Service) queueRetry(ctx context.Context, msg *Message, at time.Time, cause error) error {
	if s.retrySch == nil {
		return cause
	}

	retryAt := at.UTC().Add(messageRetryDelay)
	if _, err := s.retrySch.ScheduleAt(retryAt, func() {
		s.retryQueuedMessage(msg.ID)
	}); err != nil {
		msg.Status = StatusFailed
		msg.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, msg)
		return err
	}

	msg.Status = StatusPending
	msg.ErrorMessage = cause.Error()
	return s.store.Update(ctx, msg)
}

func (s *Service) retryQueuedMessage(id uint) {
	ctx := context.Background()
	msg, err := s.store.Get(ctx, id)
	if err != nil || msg.Status != StatusPending {
		return
	}
	_ = s.play(ctx, &msg, s.now().UTC())
}

func (s *Service) tryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	if s.voiceGate == nil {
		return nil, nil
	}
	return s.voiceGate.TryAcquireProactiveVoice(ctx, deviceID, at)
}

func messageSpeakOptions() xiaozhiclient.InjectOptions {
	autoListen := true
	return xiaozhiclient.InjectOptions{SkipLLM: true, AutoListen: &autoListen}
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Message, error) {
	filter.DeviceID = strings.TrimSpace(filter.DeviceID)
	filter.Status = Status(strings.TrimSpace(string(filter.Status)))
	return s.store.List(ctx, filter)
}

func (s *Service) ListMessageStatusSummaries(ctx context.Context, deviceID string, limit int) ([]sharedtypes.MessageStatusSummary, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	messages, err := s.store.List(ctx, ListFilter{DeviceID: deviceID})
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}

	summaries := make([]sharedtypes.MessageStatusSummary, 0, len(messages))
	for _, msg := range messages {
		summaries = append(summaries, sharedtypes.MessageStatusSummary{
			MessageID: msg.ID,
			Status:    string(msg.Status),
			QueuedAt:  msg.QueuedAt,
			PlayedAt:  msg.PlayedAt,
		})
	}
	return summaries, nil
}

func trimAndLimit(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return text
	}

	runes := []rune(text)
	return string(runes[:maxRunes])
}
