package timeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func TestTimelineMergesMessagesAndHistoryChronologically(t *testing.T) {
	base := time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC)
	reader := timelineMessageReader{items: []sharedtypes.TimelineMessage{
		{MessageID: 9, Text: "晚上过去看您", SenderDisplayName: "小兰", Status: "played", QueuedAt: base.Add(time.Minute)},
	}}
	xc := historyClient{
		history: []xiaozhiclient.HistoryMessage{
			{Role: "user", Text: "好啊", At: base.Add(2 * time.Minute)},
			{Role: "assistant", Text: "我陪您等", At: base.Add(3 * time.Minute)},
		},
	}

	resp, err := NewService(reader, xc).Get(context.Background(), Request{
		DeviceID:         "dev-001",
		ElderDisplayName: "王阿姨",
	})
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items len = %d, want 3: %+v", len(resp.Items), resp.Items)
	}
	if resp.Items[0].SourceLabel != "小兰" || resp.Items[0].Status != "played" {
		t.Fatalf("child item = %+v", resp.Items[0])
	}
	if resp.Items[1].SourceLabel != "王阿姨" || resp.Items[1].Type != ItemElderSpeech {
		t.Fatalf("elder item = %+v", resp.Items[1])
	}
	if resp.Items[2].SourceLabel != "安伴" || resp.Items[2].Type != ItemAssistantReply {
		t.Fatalf("assistant item = %+v", resp.Items[2])
	}
}

func TestTimelineKeepsMessagesWhenHistoryFails(t *testing.T) {
	base := time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC)
	reader := timelineMessageReader{items: []sharedtypes.TimelineMessage{
		{MessageID: 1, Text: "记得喝水", SenderDisplayName: "小鑫", Status: "pending", QueuedAt: base},
	}}
	resp, err := NewService(reader, failingHistoryClient{}).Get(context.Background(), Request{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].SourceLabel != "小鑫" {
		t.Fatalf("items = %+v", resp.Items)
	}
}

func TestTimelinePrefersProfileElderDisplayName(t *testing.T) {
	base := time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC)
	xc := historyClient{
		history: []xiaozhiclient.HistoryMessage{
			{Role: "user", Text: "好啊", At: base},
		},
	}
	resp, err := NewService(nil, xc, elderNameReader{name: "王阿姨"}).Get(context.Background(), Request{
		DeviceID:         "dev-001",
		ElderDisplayName: "老人",
	})
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].SourceLabel != "王阿姨" {
		t.Fatalf("items = %+v", resp.Items)
	}
}

type timelineMessageReader struct {
	items []sharedtypes.TimelineMessage
}

type elderNameReader struct {
	name string
}

func (r elderNameReader) GetElderDisplayName(context.Context, string) (string, error) {
	return r.name, nil
}

func (r timelineMessageReader) ListTimelineMessages(context.Context, string, int) ([]sharedtypes.TimelineMessage, error) {
	return r.items, nil
}

type failingHistoryClient struct{}

func (failingHistoryClient) InjectSpeak(context.Context, string, string, xiaozhiclient.InjectOptions) error {
	return nil
}
func (failingHistoryClient) GetDeviceStatus(context.Context, string) (xiaozhiclient.DeviceStatus, error) {
	return xiaozhiclient.DeviceStatus{}, nil
}
func (failingHistoryClient) GetHistory(context.Context, string, int) ([]xiaozhiclient.HistoryMessage, error) {
	return nil, errors.New("manager unavailable")
}
func (failingHistoryClient) SetRolePrompt(context.Context, string, string) error { return nil }
func (failingHistoryClient) CallDeviceMCPTool(context.Context, string, string, map[string]any) (json.RawMessage, error) {
	return nil, nil
}

type historyClient struct {
	failingHistoryClient
	history []xiaozhiclient.HistoryMessage
}

func (c historyClient) GetHistory(context.Context, string, int) ([]xiaozhiclient.HistoryMessage, error) {
	return c.history, nil
}
