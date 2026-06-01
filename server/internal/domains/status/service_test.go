package status

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestServiceGetReturnsDeviceSnapshot(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	xc := &statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
	}
	svc := NewService(xc)

	snapshot, err := svc.Get(context.Background(), GetRequest{DeviceID: " dev-001 "})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if xc.gotDeviceID != "dev-001" {
		t.Fatalf("deviceID = %q, want trimmed dev-001", xc.gotDeviceID)
	}
	if snapshot.DeviceID != "dev-001" || !snapshot.Online {
		t.Fatalf("snapshot = %+v, want dev-001 online", snapshot)
	}
	if snapshot.LastSeenAt == nil || !snapshot.LastSeenAt.Equal(lastActive) {
		t.Fatalf("lastSeenAt = %v, want %s", snapshot.LastSeenAt, lastActive)
	}
	if snapshot.LastInteractionAt == nil || !snapshot.LastInteractionAt.Equal(lastActive) {
		t.Fatalf("lastInteractionAt = %v, want %s", snapshot.LastInteractionAt, lastActive)
	}
}

func TestServiceGetRejectsMissingDeviceID(t *testing.T) {
	svc := NewService(&statusClient{})

	_, err := svc.Get(context.Background(), GetRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

type statusClient struct {
	xiaozhiclient.FakeClient
	status      xiaozhiclient.DeviceStatus
	err         error
	gotDeviceID string
}

func (c *statusClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	c.gotDeviceID = deviceID
	return c.status, c.err
}
