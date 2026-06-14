package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client) *Service {
	t.Helper()
	return newTestServiceWithScheduler(t, xc, nil)
}

func newTestServiceWithScheduler(t *testing.T, xc xiaozhiclient.Client, sch *fakeOneShotScheduler) *Service {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	msgStore := NewStore(st.DB)
	if err := msgStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	var svc *Service
	if sch == nil {
		svc = NewService(msgStore, xc)
	} else {
		svc = NewService(msgStore, xc, sch)
	}
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
	if call.Opts.AutoListen == nil || !*call.Opts.AutoListen {
		t.Fatal("AutoListen is not true; child message playback should hand control back to xiaozhi listening loop")
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

func TestServiceBypassesProactiveVoiceQuota(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	fakeSch := &fakeOneShotScheduler{}
	svc := newTestServiceWithScheduler(t, fake, fakeSch)
	svc.UseProactiveVoiceGate(throttledVoiceGate{})

	msg, err := svc.Send(context.Background(), SendRequest{
		DeviceID: "dev-001",
		Text:     "妈，我下班路过老张家了",
		FromName: "小明",
	})
	if err != nil {
		t.Fatalf("Send err = %v, want child message to bypass proactive voice quota", err)
	}
	if msg.Status != StatusPlayed {
		t.Fatalf("Status = %q, want %q", msg.Status, StatusPlayed)
	}
	if msg.ErrorMessage != "" {
		t.Fatalf("ErrorMessage = %q, want empty", msg.ErrorMessage)
	}
	if len(fake.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want direct xiaozhi injection despite throttled proactive voice gate", len(fake.InjectCalls))
	}
	if fake.InjectCalls[0].Text != "刚才小明发来留言：妈，我下班路过老张家了" {
		t.Fatalf("inject text = %q", fake.InjectCalls[0].Text)
	}
	if len(fakeSch.jobs) != 0 {
		t.Fatalf("one-shot jobs = %d, want no proactive quota retry for child messages", len(fakeSch.jobs))
	}

	list, err := svc.List(context.Background(), ListFilter{Status: StatusPlayed})
	if err != nil {
		t.Fatalf("List played: %v", err)
	}
	if len(list) != 1 || list[0].ID != msg.ID {
		t.Fatalf("played messages = %+v, want played message %d", list, msg.ID)
	}
}

func TestServiceSendMessageFailureDoesNotBlockNextMessage(t *testing.T) {
	xc := &recoveringClient{err: errors.New("manager unavailable")}
	svc := newTestService(t, xc)
	ctx := context.Background()

	failed, err := svc.Send(ctx, SendRequest{DeviceID: "dev-001", Text: "第一条先失败"})
	if err == nil {
		t.Fatal("expected first send to fail, got nil")
	}
	if failed.Status != StatusFailed {
		t.Fatalf("first status = %q, want %q", failed.Status, StatusFailed)
	}

	played, err := svc.Send(ctx, SendRequest{DeviceID: "dev-001", Text: "第二条应该继续播报"})
	if err != nil {
		t.Fatalf("second Send: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("second status = %q, want %q", played.Status, StatusPlayed)
	}
	if len(xc.successfulCalls) != 1 {
		t.Fatalf("successful inject calls = %d, want 1", len(xc.successfulCalls))
	}
	if xc.successfulCalls[0].Text != "第二条应该继续播报" {
		t.Fatalf("successful inject text = %q", xc.successfulCalls[0].Text)
	}

	messages, err := svc.List(ctx, ListFilter{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %+v, want failed and played messages", messages)
	}
	if messages[0].ID != played.ID || messages[0].Status != StatusPlayed {
		t.Fatalf("newest message = %+v, want played second message", messages[0])
	}
	if messages[1].ID != failed.ID || messages[1].Status != StatusFailed {
		t.Fatalf("older message = %+v, want failed first message", messages[1])
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

type recoveringClient struct {
	xiaozhiclient.FakeClient
	err             error
	successfulCalls []xiaozhiclient.InjectCall
}

func (c *recoveringClient) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	if c.err != nil {
		err := c.err
		c.err = nil
		return err
	}
	c.successfulCalls = append(c.successfulCalls, xiaozhiclient.InjectCall{DeviceID: deviceID, Text: text, Opts: opts})
	return nil
}

type fakeOneShotScheduler struct {
	jobs []fakeOneShotJob
}

type fakeOneShotJob struct {
	at time.Time
	fn func()
}

func (f *fakeOneShotScheduler) ScheduleAt(at time.Time, fn func()) (scheduler.JobID, error) {
	f.jobs = append(f.jobs, fakeOneShotJob{at: at, fn: fn})
	return scheduler.JobID("once-" + string(rune('0'+len(f.jobs)))), nil
}

type throttledVoiceGate struct{}

func (throttledVoiceGate) TryAcquireProactiveVoice(context.Context, string, time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	return nil, sharedtypes.ErrProactiveVoiceThrottled
}
