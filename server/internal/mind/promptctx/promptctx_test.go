package promptctx

import (
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestBuildCreatesDeterministicMindContextFromStateAndRecentEvents(t *testing.T) {
	at := time.Date(2026, 6, 16, 15, 0, 0, 0, time.UTC)
	got := Build(mind.SelfState{
		DeviceID:  "dev-001",
		At:        at,
		Concern:   0.78,
		Warmth:    0.68,
		Quietness: 0.72,
	}, []mind.Event{
		{
			ID:       "evt-user",
			DeviceID: "dev-001",
			Type:     mind.EventElderSpoke,
			At:       at.Add(-10 * time.Minute),
			Summary:  "老人说：今天有点累，想看看花",
			Payload:  map[string]any{"text": "今天有点累，想看看花"},
		},
		{
			ID:       "evt-silence",
			DeviceID: "dev-001",
			Type:     mind.EventLongSilence,
			At:       at.Add(-30 * time.Minute),
			Summary:  "午后互动偏少",
		},
	})

	for _, want := range []string{
		"concern 偏高",
		"更关切些",
		"关系温度较暖",
		"偏安静",
		"今天有点累",
		"午后互动偏少",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context = %q, want contains %q", got, want)
		}
	}
	if len([]rune(got)) > 220 {
		t.Fatalf("context length = %d, want concise <= 220", len([]rune(got)))
	}
}

func TestBuildReturnsEmptyWhenStateAndEventsHaveNoSignal(t *testing.T) {
	got := Build(mind.SelfState{DeviceID: "dev-001", Concern: 0.3, Warmth: 0.45, Quietness: 0.4}, nil)
	if got != "" {
		t.Fatalf("context = %q, want empty without meaningful signal", got)
	}
}
