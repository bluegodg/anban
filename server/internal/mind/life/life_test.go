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
