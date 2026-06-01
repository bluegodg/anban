package status

import (
	"context"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Service struct {
	xc xiaozhiclient.Client
}

func NewService(xc xiaozhiclient.Client) *Service {
	return &Service{xc: xc}
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

	lastSeen := timePtr(status.LastActiveAt)
	return Snapshot{
		DeviceID:          status.DeviceID,
		Online:            status.Online,
		LastSeenAt:        lastSeen,
		LastInteractionAt: lastSeen,
	}, nil
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}
