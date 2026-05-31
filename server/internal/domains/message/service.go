package message

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

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

	speakText := msg.Text
	if msg.FromName != "" {
		speakText = "刚才" + msg.FromName + "发来留言：" + msg.Text
	}

	if err := s.xc.InjectSpeak(ctx, msg.DeviceID, speakText, xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		msg.Status = StatusFailed
		msg.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, &msg)
		return msg, err
	}

	playedAt := s.now().UTC()
	msg.Status = StatusPlayed
	msg.PlayedAt = &playedAt
	if err := s.store.Update(ctx, &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Message, error) {
	return s.store.List(ctx, filter)
}

func trimAndLimit(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return text
	}

	runes := []rune(text)
	return string(runes[:maxRunes])
}
