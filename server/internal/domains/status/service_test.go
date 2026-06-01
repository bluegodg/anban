package status

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
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

func TestServiceGetIncludesRecentMessageStatuses(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	queued := time.Date(2026, 6, 1, 8, 29, 0, 0, time.UTC)
	played := time.Date(2026, 6, 1, 8, 29, 5, 0, time.UTC)
	reader := &statusMessageReader{
		summaries: []sharedtypes.MessageStatusSummary{
			{MessageID: 42, Status: "played", QueuedAt: queued, PlayedAt: &played},
		},
	}
	xc := &statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
	}
	svc := NewService(xc, reader)

	snapshot, err := svc.Get(context.Background(), GetRequest{DeviceID: " dev-001 "})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reader.gotDeviceID != "dev-001" {
		t.Fatalf("message reader deviceID = %q, want dev-001", reader.gotDeviceID)
	}
	if reader.gotLimit != 10 {
		t.Fatalf("message reader limit = %d, want 10", reader.gotLimit)
	}
	if len(snapshot.Messages) != 1 {
		t.Fatalf("messages = %+v, want one summary", snapshot.Messages)
	}
	if snapshot.Messages[0].MessageID != 42 || snapshot.Messages[0].Status != "played" {
		t.Fatalf("message summary = %+v, want played message 42", snapshot.Messages[0])
	}
	if snapshot.Messages[0].PlayedAt == nil || !snapshot.Messages[0].PlayedAt.Equal(played) {
		t.Fatalf("playedAt = %v, want %s", snapshot.Messages[0].PlayedAt, played)
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

type statusMessageReader struct {
	summaries   []sharedtypes.MessageStatusSummary
	err         error
	gotDeviceID string
	gotLimit    int
}

func (r *statusMessageReader) ListMessageStatusSummaries(ctx context.Context, deviceID string, limit int) ([]sharedtypes.MessageStatusSummary, error) {
	r.gotDeviceID = deviceID
	r.gotLimit = limit
	return r.summaries, r.err
}
