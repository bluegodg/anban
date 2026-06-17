package main

import (
	"context"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/executors"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestHistoryMindEventMapsConversationRoles(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		message  xiaozhiclient.HistoryMessage
		wantType mind.EventType
	}{
		{
			name:     "user",
			message:  xiaozhiclient.HistoryMessage{Role: "user", Text: "我今天想听会儿戏", At: at},
			wantType: mind.EventElderSpoke,
		},
		{
			name:     "assistant",
			message:  xiaozhiclient.HistoryMessage{Role: "assistant", Text: "好呀，我陪您慢慢听。", At: at},
			wantType: mind.EventAssistantSpoke,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := historyMindEvent("dev-001", tt.message)
			if !ok {
				t.Fatal("historyMindEvent ok = false, want true")
			}
			if event.Type != tt.wantType || event.Source != mind.SourceXiaozhi {
				t.Fatalf("event = %+v, want xiaozhi %s", event, tt.wantType)
			}
			if event.ID == "" || event.DeviceID != "dev-001" || event.Summary == "" {
				t.Fatalf("event = %+v, want stable id, device, and summary", event)
			}
			again, ok := historyMindEvent("dev-001", tt.message)
			if !ok || again.ID != event.ID {
				t.Fatalf("second event = %+v ok=%v, want deterministic id %q", again, ok, event.ID)
			}
		})
	}
}

func TestHistoryMindEventSkipsUnusableMessages(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)
	tests := []xiaozhiclient.HistoryMessage{
		{Role: "tool", Text: "internal", At: at},
		{Role: "user", Text: "   ", At: at},
		{Role: "user", Text: "没有时间", At: time.Time{}},
	}
	for _, msg := range tests {
		if event, ok := historyMindEvent("dev-001", msg); ok {
			t.Fatalf("historyMindEvent(%+v) = %+v, want skipped", msg, event)
		}
	}
}

func TestMindActionExecutorDefersMissingSpeakExecutor(t *testing.T) {
	exec := mindActionExecutor{dispatcher: executors.NewDispatcher(map[string]executors.SpeakExecutor{})}

	result, err := exec.Execute(context.Background(), mind.Action{
		ID:       "action-message",
		DeviceID: "dev-001",
		Type:     mind.ActionSpeak,
		Executor: "message",
	})
	if err != nil {
		t.Fatalf("Execute error = %v, want missing executor to be a safe defer", err)
	}
	if result.Status != mind.ActionDeferred {
		t.Fatalf("result = %+v, want deferred", result)
	}
}
