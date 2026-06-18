package memory

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/llm"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

const (
	defaultHistoryLimit = 50
	defaultMaxFacts     = 20
)

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	extractor llm.FactExtractor
	syncer    PromptSyncer
	opts      Options
}

func NewService(store *Store, xc xiaozhiclient.Client, extractor llm.FactExtractor, syncer PromptSyncer, opts Options) *Service {
	if opts.HistoryLimit <= 0 {
		opts.HistoryLimit = defaultHistoryLimit
	}
	if opts.MaxFacts <= 0 {
		opts.MaxFacts = defaultMaxFacts
	}
	return &Service{
		store:     store,
		xc:        xc,
		extractor: extractor,
		syncer:    syncer,
		opts:      opts,
	}
}

func (s *Service) DistillDevice(ctx context.Context, deviceID string) (DistillResult, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DistillResult{}, ErrInvalidInput
	}
	result := DistillResult{DeviceID: deviceID}
	if s.extractor == nil {
		result.Degraded = true
		result.DegradeNote = "llm disabled"
		return result, nil
	}

	existing, err := s.store.ListFacts(ctx, deviceID, s.opts.MaxFacts)
	if err != nil {
		return result, err
	}
	history, err := s.xc.GetHistory(ctx, deviceID, s.opts.HistoryLimit)
	if err != nil {
		return result, err
	}
	if len(history) == 0 {
		result.TotalFacts = len(existing)
		return result, s.syncFacts(ctx, deviceID, existing)
	}

	extracted, err := s.extractor.ExtractFacts(ctx, llm.FactExtractionRequest{
		DeviceID:      deviceID,
		Messages:      llmMessages(history),
		ExistingFacts: append([]string(nil), existing...),
		Limit:         s.opts.MaxFacts,
	})
	if err != nil {
		return result, err
	}

	added, err := s.store.UpsertFacts(ctx, deviceID, extracted, latestHistoryAt(history))
	if err != nil {
		return result, err
	}
	result.AddedFacts = added

	allFacts, err := s.store.ListFacts(ctx, deviceID, s.opts.MaxFacts)
	if err != nil {
		return result, err
	}
	result.TotalFacts = len(allFacts)
	if err := s.syncFacts(ctx, deviceID, allFacts); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) ListFacts(ctx context.Context, deviceID string, limit int) ([]Fact, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidInput
	}
	return s.store.ListFactItems(ctx, deviceID, limit)
}

func (s *Service) AddManualFact(ctx context.Context, req FactRequest) (Fact, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Fact{}, ErrInvalidInput
	}
	fact, err := s.store.AddManualFact(ctx, deviceID, req.Text, time.Now().UTC())
	if err != nil {
		return Fact{}, err
	}
	if err := s.syncAllFacts(ctx, deviceID); err != nil {
		return fact, err
	}
	return fact, nil
}

func (s *Service) UpdateFact(ctx context.Context, deviceID string, factID uint, req FactRequest) (Fact, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(req.DeviceID)
	}
	if deviceID == "" {
		return Fact{}, ErrInvalidInput
	}
	fact, err := s.store.UpdateFact(ctx, deviceID, factID, req.Text)
	if err != nil {
		return Fact{}, err
	}
	if err := s.syncAllFacts(ctx, deviceID); err != nil {
		return fact, err
	}
	return fact, nil
}

func (s *Service) DeleteFact(ctx context.Context, deviceID string, factID uint) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ErrInvalidInput
	}
	if err := s.store.DeleteFact(ctx, deviceID, factID); err != nil {
		return err
	}
	return s.syncAllFacts(ctx, deviceID)
}

func (s *Service) syncFacts(ctx context.Context, deviceID string, facts []string) error {
	if s.syncer == nil {
		return nil
	}
	return s.syncer.SyncMemoryFacts(ctx, deviceID, facts)
}

func (s *Service) syncAllFacts(ctx context.Context, deviceID string) error {
	if s.syncer == nil {
		return nil
	}
	facts, err := s.store.ListFacts(ctx, deviceID, s.opts.MaxFacts)
	if err != nil {
		return err
	}
	return s.syncer.SyncMemoryFacts(ctx, deviceID, facts)
}

func llmMessages(history []xiaozhiclient.HistoryMessage) []llm.Message {
	out := make([]llm.Message, 0, len(history))
	for _, msg := range history {
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}
		out = append(out, llm.Message{
			Role: strings.TrimSpace(msg.Role),
			Text: text,
			At:   msg.At,
		})
	}
	return out
}

func latestHistoryAt(history []xiaozhiclient.HistoryMessage) time.Time {
	var latest time.Time
	for _, msg := range history {
		if msg.At.After(latest) {
			latest = msg.At
		}
	}
	return latest
}
