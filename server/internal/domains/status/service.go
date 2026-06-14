package status

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const defaultMessageStatusLimit = 10
const defaultHistoryLimit = 10

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
		if message.At.IsZero() {
			continue
		}
		at := message.At.UTC()
		if latest.IsZero() || at.After(latest) {
			latest = at
		}
	}
	return timePtr(latest)
}
