package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := st.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	s := NewStore(st.DB)
	if err := s.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return s
}

// failingInject 包装 FakeClient，让 InjectSpeak 必失败，用于验证保留待播。
type failingInject struct {
	*xiaozhiclient.FakeClient
	err error
}

func (f failingInject) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	return f.err
}

func TestEnqueueThenFlushDeliversViaInnerClientOnce(t *testing.T) {
	st := newTestStore(t)
	fake := &xiaozhiclient.FakeClient{}
	svc := NewService(fake, st)
	ctx := context.Background()

	if err := svc.Enqueue(ctx, "dev-001", "你女儿留言：记得吃药", xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := svc.Enqueue(ctx, "dev-001", "早上好呀", xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}

	delivered, err := svc.Flush(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want 2", delivered)
	}
	if len(fake.InjectCalls) != 2 {
		t.Fatalf("InjectCalls = %d, want 2", len(fake.InjectCalls))
	}
	// 顺序：最旧在前
	if fake.InjectCalls[0].Text != "你女儿留言：记得吃药" {
		t.Fatalf("first call = %q, want oldest first", fake.InjectCalls[0].Text)
	}

	// 再 flush 不应重复投递
	again, err := svc.Flush(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Flush again: %v", err)
	}
	if again != 0 || len(fake.InjectCalls) != 2 {
		t.Fatalf("second flush delivered=%d calls=%d, want no re-delivery", again, len(fake.InjectCalls))
	}
}

func TestClientDecoratorEnqueuesInsteadOfInjecting(t *testing.T) {
	st := newTestStore(t)
	fake := &xiaozhiclient.FakeClient{}
	svc := NewService(fake, st)
	dec := NewClient(fake, svc)
	ctx := context.Background()

	if err := dec.InjectSpeak(ctx, "dev-001", "在吗", xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("decorator InjectSpeak: %v", err)
	}
	if len(fake.InjectCalls) != 0 {
		t.Fatalf("inner InjectCalls = %d, want 0 (should be queued, not sent)", len(fake.InjectCalls))
	}
	n, err := st.CountPending(ctx, "dev-001")
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if n != 1 {
		t.Fatalf("pending = %d, want 1", n)
	}
	// 非 InjectSpeak 方法应透传到内层
	status, err := dec.GetDeviceStatus(ctx, "dev-001")
	if err != nil || status.DeviceID != "dev-001" {
		t.Fatalf("delegate GetDeviceStatus = %+v err=%v", status, err)
	}
}

func TestFlushExpiresStaleItemsWithoutDelivering(t *testing.T) {
	st := newTestStore(t)
	fake := &xiaozhiclient.FakeClient{}
	svc := NewService(fake, st)
	base := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return base })
	ctx := context.Background()

	if err := svc.Enqueue(ctx, "dev-001", "早安", xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// 时间推到 TTL 之后
	svc.SetClock(func() time.Time { return base.Add(DefaultTTL + time.Minute) })

	delivered, err := svc.Flush(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if delivered != 0 || len(fake.InjectCalls) != 0 {
		t.Fatalf("delivered=%d calls=%d, want stale item expired not delivered", delivered, len(fake.InjectCalls))
	}
	n, _ := st.CountPending(ctx, "dev-001")
	if n != 0 {
		t.Fatalf("pending = %d, want 0 (expired)", n)
	}
}

func TestFlushKeepsPendingWhenInjectFails(t *testing.T) {
	st := newTestStore(t)
	failing := failingInject{FakeClient: &xiaozhiclient.FakeClient{}, err: errors.New("boom")}
	svc := NewService(failing, st)
	ctx := context.Background()

	if err := svc.Enqueue(ctx, "dev-001", "提醒：量血压", xiaozhiclient.InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	delivered, err := svc.Flush(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if delivered != 0 {
		t.Fatalf("delivered = %d, want 0", delivered)
	}
	n, _ := st.CountPending(ctx, "dev-001")
	if n != 1 {
		t.Fatalf("pending = %d, want 1 (kept for retry)", n)
	}
}

func TestEnqueueTrimsToCapDroppingOldest(t *testing.T) {
	st := newTestStore(t)
	fake := &xiaozhiclient.FakeClient{}
	svc := NewService(fake, st)
	svc.SetMaxPerDevice(2)
	ctx := context.Background()

	for _, txt := range []string{"一", "二", "三"} {
		if err := svc.Enqueue(ctx, "dev-001", txt, xiaozhiclient.InjectOptions{}); err != nil {
			t.Fatalf("Enqueue %s: %v", txt, err)
		}
	}
	n, _ := st.CountPending(ctx, "dev-001")
	if n != 2 {
		t.Fatalf("pending = %d, want 2 (capped)", n)
	}
	items, _ := st.ListPending(ctx, "dev-001", 0)
	if len(items) != 2 || items[0].Text != "二" || items[1].Text != "三" {
		t.Fatalf("pending = %+v, want oldest '一' dropped", items)
	}
}

func TestPendingDeviceIDsListsOnlyDevicesWithPending(t *testing.T) {
	st := newTestStore(t)
	fake := &xiaozhiclient.FakeClient{}
	svc := NewService(fake, st)
	ctx := context.Background()

	_ = svc.Enqueue(ctx, "dev-a", "x", xiaozhiclient.InjectOptions{})
	_ = svc.Enqueue(ctx, "dev-b", "y", xiaozhiclient.InjectOptions{})
	if _, err := svc.Flush(ctx, "dev-a"); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	ids, err := svc.PendingDeviceIDs(ctx)
	if err != nil {
		t.Fatalf("PendingDeviceIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != "dev-b" {
		t.Fatalf("pending devices = %v, want [dev-b]", ids)
	}
}
