package situation

import (
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

func has(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
