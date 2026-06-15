package status

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const defaultMessageStatusLimit = 10
const defaultHistoryLimit = 10
const defaultConversationHistoryLimit = 50
const maxConversationHistoryLimit = 100

type Service struct {
	xc            xiaozhiclient.Client
	messageReader sharedtypes.MessageStatusReader
	store         *Store
}

func NewService(xc xiaozhiclient.Client, readers ...sharedtypes.MessageStatusReader) *Service {
	var messageReader sharedtypes.MessageStatusReader
	if len(readers) > 0 {
		messageReader = readers[0]
	}
	return &Service{xc: xc, messageReader: messageReader}
}

func (s *Service) UseStore(store *Store) {
	s.store = store
}

func (s *Service) Get(ctx context.Context, req GetRequest) (Snapshot, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Snapshot{}, ErrInvalidInput
	}

	messages := s.messageSummaries(ctx, deviceID)
	status, err := s.xc.GetDeviceStatus(ctx, deviceID)
	if err != nil {
		if cached, cacheErr := s.cachedSnapshot(ctx, deviceID, messages); cacheErr == nil {
			return cached, nil
		}
		return Snapshot{}, err
	}
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}

	lastSeen := timePtr(status.LastActiveAt)
	lastInteraction := lastSeen
	history, err := s.xc.GetHistory(ctx, deviceID, defaultHistoryLimit)
	if err == nil {
		if at := latestHistoryAt(history); at != nil {
			lastInteraction = at
		}
	}

	snapshot := Snapshot{
		DeviceID:          status.DeviceID,
		Online:            status.Online,
		LastSeenAt:        lastSeen,
		LastInteractionAt: lastInteraction,
		Messages:          messages,
	}
	s.cacheSnapshot(ctx, snapshot)
	return snapshot, nil
}

func (s *Service) GetHistory(ctx context.Context, req HistoryRequest) (HistoryResponse, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return HistoryResponse{}, ErrInvalidInput
	}

	history, err := s.xc.GetHistory(ctx, deviceID, normalizeHistoryLimit(req.Limit))
	if err != nil {
		return HistoryResponse{}, err
	}

	messages := make([]HistoryEntry, 0, len(history))
	for _, message := range history {
		role := normalizeConversationRole(message.Role)
		text := strings.TrimSpace(message.Text)
		if role == "" || text == "" {
			continue
		}
		messages = append(messages, HistoryEntry{
			Role: role,
			Text: text,
			At:   timePtr(message.At),
		})
	}
	sortHistoryEntries(messages)
	return HistoryResponse{DeviceID: deviceID, Messages: messages}, nil
}

func (s *Service) messageSummaries(ctx context.Context, deviceID string) []sharedtypes.MessageStatusSummary {
	if s.messageReader == nil {
		return []sharedtypes.MessageStatusSummary{}
	}
	summaries, err := s.messageReader.ListMessageStatusSummaries(ctx, deviceID, defaultMessageStatusLimit)
	if err != nil {
		return []sharedtypes.MessageStatusSummary{}
	}
	return summaries
}

func (s *Service) cachedSnapshot(ctx context.Context, deviceID string, messages []sharedtypes.MessageStatusSummary) (Snapshot, error) {
	if s.store == nil {
		return Snapshot{}, ErrNotFound
	}
	cache, err := s.store.Get(ctx, deviceID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		DeviceID:          cache.DeviceID,
		Online:            false,
		LastSeenAt:        cloneTimePtr(cache.LastSeenAt),
		LastInteractionAt: cloneTimePtr(cache.LastInteractionAt),
		Messages:          messages,
	}, nil
}

func (s *Service) cacheSnapshot(ctx context.Context, snapshot Snapshot) {
	if s.store == nil || snapshot.DeviceID == "" {
		return
	}
	_ = s.store.Upsert(ctx, &SnapshotCache{
		DeviceID:          snapshot.DeviceID,
		Online:            snapshot.Online,
		LastSeenAt:        cloneTimePtr(snapshot.LastSeenAt),
		LastInteractionAt: cloneTimePtr(snapshot.LastInteractionAt),
	})
}

func normalizeHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultConversationHistoryLimit
	}
	if limit > maxConversationHistoryLimit {
		return maxConversationHistoryLimit
	}
	return limit
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func latestHistoryAt(history []xiaozhiclient.HistoryMessage) *time.Time {
	var latest time.Time
	for _, message := range history {
		if normalizeConversationRole(message.Role) == "" || strings.TrimSpace(message.Text) == "" || message.At.IsZero() {
			continue
		}
		at := message.At.UTC()
		if latest.IsZero() || at.After(latest) {
			latest = at
		}
	}
	return timePtr(latest)
}

func normalizeConversationRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return "user"
	case "assistant":
		return "assistant"
	default:
		return ""
	}
}

func sortHistoryEntries(messages []HistoryEntry) {
	sort.SliceStable(messages, func(i, j int) bool {
		if messages[i].At == nil {
			return false
		}
		if messages[j].At == nil {
			return true
		}
		return messages[i].At.Before(*messages[j].At)
	})
}
