package reflection

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestSummarizeAcceptedInteractionRaisesWarmth(t *testing.T) {
	at := time.Date(2026, 6, 16, 20, 0, 0, 0, time.UTC)
	got := Summarize("dev-001", at, []mind.Feedback{{ID: "fb-1", Signal: "user_replied", Notes: "老人接着聊了三轮"}})
	if got.StateAdjustments["warmth"] <= 0 {
		t.Fatalf("StateAdjustments = %+v, want warmth increase", got.StateAdjustments)
	}
	if len(got.BehaviorLessons) == 0 {
		t.Fatalf("BehaviorLessons empty")
	}
}
