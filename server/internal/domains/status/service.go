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
}

func NewService(xc xiaozhiclient.Client, readers ...sharedtypes.MessageStatusReader) *Service {
	var messageReader sharedtypes.MessageStatusReader
	if len(readers) > 0 {
		messageReader = readers[0]
	}
	return &Service{xc: xc, messageReader: messageReader}
}

func (s *Service) Get(ctx context.Context, req GetRequest) (Snapshot, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Snapshot{}, ErrInvalidInput
	}

	status, err := s.xc.GetDeviceStatus(ctx, deviceID)
	if err != nil {
		return Snapshot{}, err
	}
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}

	messages := []sharedtypes.MessageStatusSummary{}
	if s.messageReader != nil {
		messages, err = s.messageReader.ListMessageStatusSummaries(ctx, deviceID, defaultMessageStatusLimit)
		if err != nil {
			return Snapshot{}, err
		}
	}

	lastSeen := timePtr(status.LastActiveAt)
	lastInteraction := lastSeen
	history, err := s.xc.GetHistory(ctx, deviceID, defaultHistoryLimit)
	if err == nil {
		if at := latestHistoryAt(history); at != nil {
			lastInteraction = at
		}
	}

	return Snapshot{
		DeviceID:          status.DeviceID,
		Online:            status.Online,
		LastSeenAt:        lastSeen,
		LastInteractionAt: lastInteraction,
		Messages:          messages,
	}, nil
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
