package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client) *Service {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	msgStore := NewStore(st.DB)
	if err := msgStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	svc := NewService(msgStore, xc)
	svc.now = func() time.Time { return time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC) }
	return svc
}

func TestServiceSendMessageInjectsAndPersistsPlayed(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fake)

	got, err := svc.Send(context.Background(), SendRequest{
		DeviceID: "dev-001",
		Text:     "妈，我下班路过老张家了，他说让你有空过去喝茶。",
		FromName: "小明",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("message ID was not assigned")
	}
	if got.Status != StatusPlayed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusPlayed)
	}
	if got.PlayedAt == nil {
		t.Fatal("PlayedAt is nil")
	}
	if len(fake.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want 1", len(fake.InjectCalls))
	}
	call := fake.InjectCalls[0]
	if call.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want dev-001", call.DeviceID)
	}
	if call.Text != "刚才小明发来留言：妈，我下班路过老张家了，他说让你有空过去喝茶。" {
		t.Fatalf("inject text = %q", call.Text)
	}
	if !call.Opts.SkipLLM {
		t.Fatal("SkipLLM = false, want true for exact child message playback")
	}

	list, err := svc.List(context.Background(), ListFilter{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("List = %+v, want the created message", list)
	}
}

func TestServiceSendMessageValidatesAndTruncatesText(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{})

	if _, err := svc.Send(context.Background(), SendRequest{DeviceID: "", Text: "hello"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing deviceID err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.Send(context.Background(), SendRequest{DeviceID: "dev-001", Text: "   "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank text err = %v, want ErrInvalidInput", err)
	}

	longText := ""
	for i := 0; i < 120; i++ {
		longText += "好"
	}
	msg, err := svc.Send(context.Background(), SendRequest{DeviceID: "dev-001", Text: longText})
	if err != nil {
		t.Fatalf("Send long text: %v", err)
	}
	if got := len([]rune(msg.Text)); got != MaxTextRunes {
		t.Fatalf("stored rune length = %d, want %d", got, MaxTextRunes)
	}
}

func TestServiceMarksMessageFailedWhenInjectFails(t *testing.T) {
	svc := newTestService(t, failingClient{err: errors.New("manager unavailable")})

	msg, err := svc.Send(context.Background(), SendRequest{DeviceID: "dev-001", Text: "今晚记得吃饭"})
	if err == nil {
		t.Fatal("expected inject error, got nil")
	}
	if msg.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", msg.Status, StatusFailed)
	}
	if msg.ErrorMessage == "" {
		t.Fatal("ErrorMessage is empty")
	}

	list, err := svc.List(context.Background(), ListFilter{Status: StatusFailed})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].Status != StatusFailed {
		t.Fatalf("failed list = %+v", list)
	}
}

func TestServiceListMessageStatusSummaries(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	ctx := context.Background()
	olderQueued := time.Date(2026, 6, 1, 8, 10, 0, 0, time.UTC)
	newerQueued := time.Date(2026, 6, 1, 8, 20, 0, 0, time.UTC)
	played := newerQueued.Add(5 * time.Second)

	older := Message{DeviceID: "dev-001", Text: "早一点的留言", Status: StatusPlayed, QueuedAt: olderQueued, PlayedAt: &olderQueued}
	if err := svc.store.Create(ctx, &older); err != nil {
		t.Fatalf("create older: %v", err)
	}
	newer := Message{DeviceID: "dev-001", Text: "新留言", Status: StatusPlayed, QueuedAt: newerQueued, PlayedAt: &played}
	if err := svc.store.Create(ctx, &newer); err != nil {
		t.Fatalf("create newer: %v", err)
	}
	otherDevice := Message{DeviceID: "dev-002", Text: "别的设备", Status: StatusPlayed, QueuedAt: newerQueued}
	if err := svc.store.Create(ctx, &otherDevice); err != nil {
		t.Fatalf("create other device: %v", err)
	}

	got, err := svc.ListMessageStatusSummaries(ctx, "dev-001", 1)
	if err != nil {
		t.Fatalf("ListMessageStatusSummaries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("summaries = %+v, want one newest summary", got)
	}
	if got[0].MessageID != newer.ID || got[0].Status != string(StatusPlayed) {
		t.Fatalf("summary = %+v, want newest played message", got[0])
	}
	if !got[0].QueuedAt.Equal(newerQueued) {
		t.Fatalf("queuedAt = %s, want %s", got[0].QueuedAt, newerQueued)
	}
	if got[0].PlayedAt == nil || !got[0].PlayedAt.Equal(played) {
		t.Fatalf("playedAt = %v, want %s", got[0].PlayedAt, played)
	}
}

type failingClient struct {
	err error
}

func (f failingClient) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	return f.err
}

func (f failingClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	return xiaozhiclient.DeviceStatus{}, nil
}

func (f failingClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]xiaozhiclient.HistoryMessage, error) {
	return nil, nil
}

func (f failingClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	return nil
}

func (f failingClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
