package situation

import (
	"slices"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestBuildMarksNightLongSilenceAsQuietObservation(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 10, 0, 0, time.UTC)
	got := Build("dev-001", at, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-time.Minute), Summary: "老人 50 分钟无互动"},
	})
	if got.TimeOfDay != "night" {
		t.Fatalf("TimeOfDay = %q, want night", got.TimeOfDay)
	}
	if got.InteractionMode != "idle" || got.ActivityLevel != "low" {
		t.Fatalf("situation = %+v, want idle low activity", got)
	}
	if !has(got.Constraints, "avoid_long_speech") || !has(got.Constraints, "prefer_observation") {
		t.Fatalf("constraints = %+v, want avoid_long_speech and prefer_observation", got.Constraints)
	}
}

func TestBuildDetectsChildMessageSocialContext(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	got := Build("dev-001", at, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at, Summary: "小明发来留言"},
	})
	if got.SocialContext != "child_waiting_reply" {
		t.Fatalf("SocialContext = %q, want child_waiting_reply", got.SocialContext)
	}
	if !has(got.OpenLoops, "child_message_pending") {
		t.Fatalf("OpenLoops = %+v, want child_message_pending", got.OpenLoops)
	}
}

func TestBuildAppliesRecentFirstEventsChronologically(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	events := []mind.Event{
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventElderSpoke, At: at, Summary: "老人刚刚回应了"},
		{ID: "evt-old", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-30 * time.Minute), Summary: "之前长时间安静"},
	}

	got := Build("dev-001", at, events)

	if got.InteractionMode != "conversation" || got.ActivityLevel != "normal" {
		t.Fatalf("situation = %+v, want latest elder spoke to win conversation normal", got)
	}
	if !slices.Equal([]string{events[0].ID, events[1].ID}, []string{"evt-new", "evt-old"}) {
		t.Fatalf("events mutated to %+v, want caller order preserved", events)
	}
}

func TestBuildDedupesConstraintsAndOpenLoops(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	got := Build("dev-001", at, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-3 * time.Minute)},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-2 * time.Minute)},
		{ID: "evt-3", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at.Add(-time.Minute)},
		{ID: "evt-4", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at},
		{ID: "evt-5", DeviceID: "dev-001", Type: mind.EventReminderDue, At: at.Add(time.Minute)},
		{ID: "evt-6", DeviceID: "dev-001", Type: mind.EventReminderDue, At: at.Add(2 * time.Minute)},
	})

	if count(got.Constraints, "avoid_long_speech") != 1 || count(got.Constraints, "prefer_observation") != 1 {
		t.Fatalf("constraints = %+v, want deduped long silence constraints", got.Constraints)
	}
	if count(got.OpenLoops, "child_message_pending") != 1 || count(got.OpenLoops, "reminder_due") != 1 {
		t.Fatalf("open loops = %+v, want deduped child message and reminder loops", got.OpenLoops)
	}
}

func TestTimeOfDayBoundariesUseTimestampLocation(t *testing.T) {
	loc := time.FixedZone("device", 8*60*60)
	tests := []struct {
		hour int
		want string
	}{
		{10, "morning"},
		{11, "noon"},
		{13, "noon"},
		{14, "afternoon"},
		{17, "afternoon"},
		{18, "evening"},
		{21, "evening"},
		{22, "night"},
	}
	for _, tt := range tests {
		at := time.Date(2026, 6, 16, tt.hour, 0, 0, 0, loc)
		if got := timeOfDay(at); got != tt.want {
			t.Fatalf("timeOfDay(%02d:00 device-local) = %q, want %q", tt.hour, got, tt.want)
		}
	}
}

func has(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func count(values []string, want string) int {
	total := 0
	for _, value := range values {
		if value == want {
			total++
		}
	}
	return total
}
