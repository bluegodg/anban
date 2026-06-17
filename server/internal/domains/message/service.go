package message

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const messageInjectTimeout = 60 * time.Second

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
}

type Service struct {
	store    *Store
	xc       xiaozhiclient.Client
	now      func() time.Time
	mindSink MindSink
}

func NewService(store *Store, xc xiaozhiclient.Client, schedulers ...OneShotScheduler) *Service {
	return &Service{
		store: store,
		xc:    xc,
		now:   time.Now,
	}
}

func (s *Service) UseProactiveVoiceGate(_ sharedtypes.ProactiveVoiceGate) {
	// Child messages are point-to-point and must not be throttled by the proactive voice quota.
}

func (s *Service) UseMindSink(sink MindSink) {
	s.mindSink = sink
}

func (s *Service) Send(ctx context.Context, req SendRequest) (Message, error) {
	msg, err := s.Queue(ctx, req)
	if err != nil {
		return msg, err
	}
	if s.mindSink != nil {
		return msg, nil
	}
	return s.PlayQueued(ctx, msg.ID)
}

func (s *Service) Queue(ctx context.Context, req SendRequest) (Message, error) {
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
	if s.mindSink != nil {
		if err := s.mindSink.IngestMindEvent(ctx, MindEvent{
			DeviceID: msg.DeviceID,
			Type:     "child_message_received",
			SourceID: msg.ID,
			Summary:  "子女留言已进入安伴心智",
			Payload:  map[string]any{"messageId": float64(msg.ID), "fromName": msg.FromName},
		}); err != nil {
			return msg, err
		}
	}
	return msg, nil
}

func (s *Service) PlayQueued(ctx context.Context, id uint) (Message, error) {
	if id == 0 {
		return Message{}, ErrInvalidInput
	}
	msg, err := s.store.Get(ctx, id)
	if err != nil {
		return Message{}, err
	}
	if msg.Status == StatusPlayed {
		return msg, nil
	}
	if err := s.play(ctx, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func (s *Service) play(ctx context.Context, msg *Message) error {
	speakText := msg.Text
	if msg.FromName != "" {
		speakText = "刚才" + msg.FromName + "发来留言：" + msg.Text
	}

	injectCtx, cancel := withMessageInjectTimeout(ctx)
	defer cancel()

	if err := s.xc.InjectSpeak(injectCtx, msg.DeviceID, speakText, messageSpeakOptions()); err != nil {
		msg.Status = StatusFailed
		msg.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, msg)
		return err
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

func withMessageInjectTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= messageInjectTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, messageInjectTimeout)
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
