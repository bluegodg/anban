package behavior

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestSelectTurnsReminderThoughtIntoSpeakAction(t *testing.T) {
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	got := Select(
		mind.Situation{DeviceID: "dev-001", At: at, InteractionMode: "idle"},
		mind.SelfState{FamilyWeight: 0.6, StewardWeight: 0.15},
		[]mind.Thought{{
			ID:               "thought-1",
			DeviceID:         "dev-001",
			At:               at,
			DriveName:        mind.DriveStewardship,
			Content:          "提醒到期",
			Urgency:          0.8,
			CareValue:        0.7,
			InterruptionCost: 0.2,
		}},
	)
	if len(got) != 1 {
		t.Fatalf("actions = %+v, want 1", got)
	}
	if got[0].Type != mind.ActionSpeak || got[0].Executor != "reminder" {
		t.Fatalf("action = %+v, want reminder speak", got[0])
	}
	if got[0].ID != "action-thought-1" || got[0].IntentionID != "intention-thought-1" {
		t.Fatalf("action identifiers = %+v, want derived IDs", got[0])
	}
	if got[0].DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want thought device", got[0].DeviceID)
	}
}

func TestSelectTurnsNightSilenceIntoWait(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Select(
		mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "night", Constraints: []string{"prefer_observation"}},
		mind.SelfState{Quietness: 0.85, FamilyWeight: 0.6},
		[]mind.Thought{{
			ID:               "thought-1",
			DeviceID:         "dev-001",
			At:               at,
			DriveName:        mind.DriveQuietPresence,
			Content:          "安静陪着",
			CareValue:        0.7,
			InterruptionCost: 0.8,
		}},
	)
	if len(got) != 1 {
		t.Fatalf("actions = %+v, want 1", got)
	}
	if got[0].Type != mind.ActionWait {
		t.Fatalf("action = %+v, want wait", got[0])
	}
	if got[0].Score <= 0 {
		t.Fatalf("score = %.2f, want positive", got[0].Score)
	}
}
