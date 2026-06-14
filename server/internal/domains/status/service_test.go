package status

import (
	"context"
	"errors"
	"testing"
	"time"

	appstore "github.com/bluegodg/anban/server/internal/store"
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

func TestServiceGetKeepsDeviceStatusWhenMessageSummaryFails(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	reader := &statusMessageReader{err: errors.New("message db timeout")}
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
	if snapshot.DeviceID != "dev-001" || !snapshot.Online {
		t.Fatalf("snapshot = %+v, want device status despite message summary failure", snapshot)
	}
	if len(snapshot.Messages) != 0 {
		t.Fatalf("messages = %+v, want empty fallback when message summary fails", snapshot.Messages)
	}
	if snapshot.LastSeenAt == nil || !snapshot.LastSeenAt.Equal(lastActive) {
		t.Fatalf("lastSeenAt = %v, want %s", snapshot.LastSeenAt, lastActive)
	}
}

func TestServiceGetUsesLatestHistoryForLastInteraction(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	olderInteraction := time.Date(2026, 6, 1, 8, 29, 0, 0, time.UTC)
	latestInteraction := time.Date(2026, 6, 1, 8, 45, 0, 0, time.UTC)
	xc := &statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
		history: []xiaozhiclient.HistoryMessage{
			{Role: "assistant", Text: "小宝今天 7 岁啦", At: olderInteraction},
			{Role: "user", Text: "今天腰有点酸", At: latestInteraction},
			{Role: "tool", Text: "self.screen.set_theme completed", At: latestInteraction.Add(time.Hour)},
		},
	}
	svc := NewService(xc)

	snapshot, err := svc.Get(context.Background(), GetRequest{DeviceID: " dev-001 "})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if xc.gotHistoryDeviceID != "dev-001" {
		t.Fatalf("history deviceID = %q, want trimmed dev-001", xc.gotHistoryDeviceID)
	}
	if xc.gotHistoryLimit != 10 {
		t.Fatalf("history limit = %d, want 10", xc.gotHistoryLimit)
	}
	if snapshot.LastSeenAt == nil || !snapshot.LastSeenAt.Equal(lastActive) {
		t.Fatalf("lastSeenAt = %v, want device last active %s", snapshot.LastSeenAt, lastActive)
	}
	if snapshot.LastInteractionAt == nil || !snapshot.LastInteractionAt.Equal(latestInteraction) {
		t.Fatalf("lastInteractionAt = %v, want latest history %s", snapshot.LastInteractionAt, latestInteraction)
	}
}

func TestServiceGetHistoryReturnsConversationMessages(t *testing.T) {
	askedAt := time.Date(2026, 6, 1, 8, 31, 0, 0, time.FixedZone("CST", 8*60*60))
	answeredAt := time.Date(2026, 6, 1, 8, 31, 5, 0, time.UTC)
	xc := &statusClient{
		history: []xiaozhiclient.HistoryMessage{
			{Role: "system", Text: "家庭画像提示词", At: askedAt.Add(-time.Second)},
			{Role: "user", Text: "我孙子叫啥", At: askedAt},
			{Role: "assistant", Text: "小宝今天 7 岁啦", At: answeredAt},
			{Role: "tool", Text: "self.screen.set_theme completed", At: answeredAt.Add(time.Second)},
		},
	}
	svc := NewService(xc)

	history, err := svc.GetHistory(context.Background(), HistoryRequest{DeviceID: " dev-001 ", Limit: 4})
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if xc.gotHistoryDeviceID != "dev-001" {
		t.Fatalf("history deviceID = %q, want trimmed dev-001", xc.gotHistoryDeviceID)
	}
	if xc.gotHistoryLimit != 4 {
		t.Fatalf("history limit = %d, want 4", xc.gotHistoryLimit)
	}
	if history.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want dev-001", history.DeviceID)
	}
	if len(history.Messages) != 2 {
		t.Fatalf("messages = %+v, want 2 conversation messages", history.Messages)
	}
	if history.Messages[0].Role != "user" || history.Messages[0].Text != "我孙子叫啥" {
		t.Fatalf("messages[0] = %+v, want user question", history.Messages[0])
	}
	if history.Messages[0].At == nil || !history.Messages[0].At.Equal(askedAt.UTC()) {
		t.Fatalf("messages[0].At = %v, want UTC %s", history.Messages[0].At, askedAt.UTC())
	}
	if history.Messages[1].Role != "assistant" || history.Messages[1].Text != "小宝今天 7 岁啦" {
		t.Fatalf("messages[1] = %+v, want assistant answer", history.Messages[1])
	}
	if history.Messages[1].At == nil || !history.Messages[1].At.Equal(answeredAt) {
		t.Fatalf("messages[1].At = %v, want %s", history.Messages[1].At, answeredAt)
	}
}

func TestServiceGetHistoryDefaultsAndCapsLimit(t *testing.T) {
	xc := &statusClient{}
	svc := NewService(xc)

	if _, err := svc.GetHistory(context.Background(), HistoryRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("GetHistory default limit: %v", err)
	}
	if xc.gotHistoryLimit != 50 {
		t.Fatalf("default history limit = %d, want 50", xc.gotHistoryLimit)
	}

	if _, err := svc.GetHistory(context.Background(), HistoryRequest{DeviceID: "dev-001", Limit: 999}); err != nil {
		t.Fatalf("GetHistory capped limit: %v", err)
	}
	if xc.gotHistoryLimit != 100 {
		t.Fatalf("capped history limit = %d, want 100", xc.gotHistoryLimit)
	}
}

func TestServiceGetHistoryRejectsMissingDeviceID(t *testing.T) {
	svc := NewService(&statusClient{})

	_, err := svc.GetHistory(context.Background(), HistoryRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetFallsBackWhenHistoryIsUnavailable(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	xc := &statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
		historyErr: sharedtypes.ErrNotImplemented,
	}
	svc := NewService(xc)

	snapshot, err := svc.Get(context.Background(), GetRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if snapshot.LastInteractionAt == nil || !snapshot.LastInteractionAt.Equal(lastActive) {
		t.Fatalf("lastInteractionAt = %v, want fallback last active %s", snapshot.LastInteractionAt, lastActive)
	}
}

func TestServiceGetKeepsDeviceStatusWhenHistoryReadFails(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	xc := &statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
		historyErr: errors.New("history api timeout"),
	}
	svc := NewService(xc)

	snapshot, err := svc.Get(context.Background(), GetRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if snapshot.DeviceID != "dev-001" || !snapshot.Online {
		t.Fatalf("snapshot = %+v, want online dev-001 despite history failure", snapshot)
	}
	if snapshot.LastInteractionAt == nil || !snapshot.LastInteractionAt.Equal(lastActive) {
		t.Fatalf("lastInteractionAt = %v, want fallback last active %s", snapshot.LastInteractionAt, lastActive)
	}
}

func TestServiceGetFallsBackToCachedSnapshotWhenDeviceStatusFails(t *testing.T) {
	statusStore := newTestStatusStore(t)
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	latestInteraction := time.Date(2026, 6, 1, 8, 45, 0, 0, time.UTC)
	playedAt := time.Date(2026, 6, 1, 8, 50, 0, 0, time.UTC)

	first := NewService(&statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
		history: []xiaozhiclient.HistoryMessage{
			{Role: "user", Text: "今天腰有点酸", At: latestInteraction},
		},
	})
	first.UseStore(statusStore)
	if _, err := first.Get(context.Background(), GetRequest{DeviceID: " dev-001 "}); err != nil {
		t.Fatalf("prime cache Get: %v", err)
	}

	reader := &statusMessageReader{
		summaries: []sharedtypes.MessageStatusSummary{
			{MessageID: 7, Status: "played", QueuedAt: playedAt.Add(-time.Minute), PlayedAt: &playedAt},
		},
	}
	failingClient := &statusClient{err: errors.New("manager unavailable")}
	second := NewService(failingClient, reader)
	second.UseStore(statusStore)

	snapshot, err := second.Get(context.Background(), GetRequest{DeviceID: " dev-001 "})
	if err != nil {
		t.Fatalf("cached fallback Get: %v", err)
	}
	if failingClient.gotDeviceID != "dev-001" {
		t.Fatalf("deviceID = %q, want trimmed dev-001", failingClient.gotDeviceID)
	}
	if snapshot.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want cached dev-001", snapshot.DeviceID)
	}
	if snapshot.Online {
		t.Fatal("Online = true, want cached fallback to report offline while manager is unavailable")
	}
	if snapshot.LastSeenAt == nil || !snapshot.LastSeenAt.Equal(lastActive) {
		t.Fatalf("LastSeenAt = %v, want cached %s", snapshot.LastSeenAt, lastActive)
	}
	if snapshot.LastInteractionAt == nil || !snapshot.LastInteractionAt.Equal(latestInteraction) {
		t.Fatalf("LastInteractionAt = %v, want cached %s", snapshot.LastInteractionAt, latestInteraction)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].MessageID != 7 {
		t.Fatalf("Messages = %+v, want current persisted message summary on cached status", snapshot.Messages)
	}
}

func newTestStatusStore(t *testing.T) *Store {
	t.Helper()
	st, err := appstore.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	statusStore := NewStore(st.DB)
	if err := statusStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate status store: %v", err)
	}
	return statusStore
}

type statusClient struct {
	xiaozhiclient.FakeClient
	status             xiaozhiclient.DeviceStatus
	history            []xiaozhiclient.HistoryMessage
	err                error
	historyErr         error
	gotDeviceID        string
	gotHistoryDeviceID string
	gotHistoryLimit    int
}

func (c *statusClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	c.gotDeviceID = deviceID
	return c.status, c.err
}

func (c *statusClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]xiaozhiclient.HistoryMessage, error) {
	c.gotHistoryDeviceID = deviceID
	c.gotHistoryLimit = limit
	return c.history, c.historyErr
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
