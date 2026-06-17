package thoughts

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestGenerateLongSilenceThoughtPrefersWaitWhenInterruptionCostHigh(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "night", Constraints: []string{"prefer_observation"}},
		mind.SelfState{Concern: 0.7, Quietness: 0.8},
		[]mind.Drive{{Name: mind.DriveCare, Strength: 0.7}, {Name: mind.DriveQuietPresence, Strength: 0.8}},
		[]mind.Event{{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at}},
	)
	if len(got) == 0 {
		t.Fatal("thoughts empty")
	}
	if got[0].DriveName != mind.DriveQuietPresence {
		t.Fatalf("DriveName = %q, want quiet_presence", got[0].DriveName)
	}
	if got[0].InterruptionCost < 0.7 {
		t.Fatalf("InterruptionCost = %.2f, want high", got[0].InterruptionCost)
	}
}

func TestGenerateChildMessageThoughtUsesFamilyBridge(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at, SocialContext: "child_waiting_reply"},
		mind.SelfState{Warmth: 0.6},
		[]mind.Drive{{Name: mind.DriveFamilyBridge, Strength: 0.8}},
		[]mind.Event{{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at}},
	)
	if len(got) != 1 {
		t.Fatalf("thoughts = %+v, want 1", got)
	}
	if got[0].DriveName != mind.DriveFamilyBridge || got[0].CareValue < 0.7 {
		t.Fatalf("thought = %+v, want family bridge high care", got[0])
	}
}
