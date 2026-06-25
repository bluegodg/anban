package life

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestUpdateCreatesThemeFromCareFocus(t *testing.T) {
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	got := Update("dev-001", at, mind.SelfState{Concern: 0.75, Playfulness: 0.2}, []mind.Event{{ID: "evt-1", Type: mind.EventLongSilence, Summary: "上午互动少"}})
	if got.TodayTheme == "" {
		t.Fatal("TodayTheme is empty")
	}
	if got.CareFocus == "" {
		t.Fatal("CareFocus is empty")
	}
	if got.PlayfulnessTrend != 0.2 {
		t.Fatalf("PlayfulnessTrend = %.2f, want 0.2", got.PlayfulnessTrend)
	}
}

func TestUpdateDedupesAndCapsLingeringThoughts(t *testing.T) {
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	events := make([]mind.Event, 0, 16)
	for i := 0; i < 16; i++ {
		events = append(events, mind.Event{ID: "evt", Type: mind.EventLongSilence, Summary: "空闲循环检测到一段沉默"})
	}
	got := Update("dev-001", at, mind.SelfState{}, events)
	if len(got.LingeringThoughts) != 1 {
		t.Fatalf("LingeringThoughts = %d (%v), want 1 deduped entry", len(got.LingeringThoughts), got.LingeringThoughts)
	}

	mixed := []mind.Event{
		{Type: mind.EventLongSilence, Summary: "沉默A"},
		{Type: mind.EventChildMessageReceived},
		{Type: mind.EventLongSilence, Summary: "沉默B"},
		{Type: mind.EventLongSilence, Summary: "沉默C"},
		{Type: mind.EventLongSilence, Summary: "沉默D"},
	}
	capped := Update("dev-001", at, mind.SelfState{}, mixed)
	if len(capped.LingeringThoughts) > maxLingeringThoughts {
		t.Fatalf("LingeringThoughts = %d, want <= %d", len(capped.LingeringThoughts), maxLingeringThoughts)
	}
}
