package reminder

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestAccountReminderMutationIsScopedToBoundDevice(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	svc.now = func() time.Time { return time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC) }
	rem, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-a",
		ScheduledAt: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC),
		Content:     "吃药",
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if _, err := svc.CancelForDevice(context.Background(), "dev-b", rem.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CancelForDevice error = %v, want ErrNotFound", err)
	}
	if _, err := svc.AcknowledgeForDevice(context.Background(), "dev-b", rem.ID, AckRequest{AckKind: AckKindVoice}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AcknowledgeForDevice error = %v, want ErrNotFound", err)
	}
}
