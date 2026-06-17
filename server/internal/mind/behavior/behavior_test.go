package behavior

import (
	"math"
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

func TestSelectKeepsUniqueThoughtIdentifiersAndGeneratesFallbacksForMissingOrDuplicateIDs(t *testing.T) {
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	got := Select(
		mind.Situation{DeviceID: "dev-fallback", At: at, InteractionMode: "idle"},
		mind.SelfState{},
		[]mind.Thought{
			{
				ID:        "thought-unique",
				DeviceID:  "dev-001",
				At:        at,
				DriveName: mind.DriveStewardship,
			},
			{
				DeviceID:  "dev-002",
				At:        at.Add(time.Minute),
				DriveName: mind.DriveCompanionship,
			},
			{
				ID:        "thought-dup",
				DeviceID:  "dev-003",
				At:        at.Add(2 * time.Minute),
				DriveName: mind.DriveFamilyBridge,
			},
			{
				ID:        "thought-dup",
				DeviceID:  "dev-004",
				At:        at.Add(3 * time.Minute),
				DriveName: mind.DriveFamilyBridge,
			},
		},
	)

	if len(got) != 4 {
		t.Fatalf("actions = %+v, want 4", got)
	}
	if got[0].ID != "action-thought-unique" || got[0].IntentionID != "intention-thought-unique" {
		t.Fatalf("unique identifiers = %+v, want original thought ID", got[0])
	}

	actionIDs := map[string]bool{}
	intentionIDs := map[string]bool{}
	for _, action := range got {
		if action.ID == "action-" || action.IntentionID == "intention-" {
			t.Fatalf("action identifiers = %+v, want non-empty fallback identifiers", action)
		}
		if actionIDs[action.ID] {
			t.Fatalf("duplicate action ID %q in %+v", action.ID, got)
		}
		if intentionIDs[action.IntentionID] {
			t.Fatalf("duplicate intention ID %q in %+v", action.IntentionID, got)
		}
		actionIDs[action.ID] = true
		intentionIDs[action.IntentionID] = true
	}

	if got[2].ID == "action-thought-dup" || got[3].ID == "action-thought-dup" {
		t.Fatalf("duplicate thought IDs should use fallback action IDs: %+v", got)
	}
	if got[2].IntentionID == "intention-thought-dup" || got[3].IntentionID == "intention-thought-dup" {
		t.Fatalf("duplicate thought IDs should use fallback intention IDs: %+v", got)
	}
}

func TestSelectUsesSituationDeviceIDWhenThoughtDeviceIDIsEmpty(t *testing.T) {
	got := Select(
		mind.Situation{DeviceID: "dev-from-situation", InteractionMode: "idle"},
		mind.SelfState{},
		[]mind.Thought{{
			ID:        "thought-1",
			DriveName: mind.DriveStewardship,
		}},
	)
	if len(got) != 1 {
		t.Fatalf("actions = %+v, want 1", got)
	}
	if got[0].DeviceID != "dev-from-situation" {
		t.Fatalf("DeviceID = %q, want situation device", got[0].DeviceID)
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

func TestSelectSchedulesCareRecheckFromAvailableBaseTime(t *testing.T) {
	situationAt := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	thoughtAt := time.Date(2026, 6, 16, 12, 5, 0, 0, time.UTC)

	t.Run("uses situation time first", func(t *testing.T) {
		got := Select(
			mind.Situation{DeviceID: "dev-001", At: situationAt},
			mind.SelfState{},
			[]mind.Thought{{
				ID:        "thought-care",
				DeviceID:  "dev-001",
				At:        thoughtAt,
				DriveName: mind.DriveCare,
			}},
		)
		if len(got) != 1 {
			t.Fatalf("actions = %+v, want 1", got)
		}
		if got[0].Type != mind.ActionScheduleRecheck || got[0].Executor != "mind" {
			t.Fatalf("action = %+v, want schedule_recheck/mind", got[0])
		}
		want := situationAt.Add(20 * time.Minute)
		if got[0].ScheduledFor == nil || !got[0].ScheduledFor.Equal(want) {
			t.Fatalf("ScheduledFor = %v, want %v", got[0].ScheduledFor, want)
		}
	})

	t.Run("falls back to thought time", func(t *testing.T) {
		got := Select(
			mind.Situation{DeviceID: "dev-001"},
			mind.SelfState{},
			[]mind.Thought{{
				ID:        "thought-care",
				DeviceID:  "dev-001",
				At:        thoughtAt,
				DriveName: mind.DriveCare,
			}},
		)
		if len(got) != 1 {
			t.Fatalf("actions = %+v, want 1", got)
		}
		want := thoughtAt.Add(20 * time.Minute)
		if got[0].ScheduledFor == nil || !got[0].ScheduledFor.Equal(want) {
			t.Fatalf("ScheduledFor = %v, want %v", got[0].ScheduledFor, want)
		}
	})

	t.Run("does not schedule zero time", func(t *testing.T) {
		got := Select(
			mind.Situation{DeviceID: "dev-001"},
			mind.SelfState{},
			[]mind.Thought{{
				ID:        "thought-care",
				DeviceID:  "dev-001",
				DriveName: mind.DriveCare,
			}},
		)
		if len(got) != 1 {
			t.Fatalf("actions = %+v, want 1", got)
		}
		if got[0].ScheduledFor != nil {
			t.Fatalf("ScheduledFor = %v, want nil", got[0].ScheduledFor)
		}
	})
}

func TestSelectRoutesDriveActions(t *testing.T) {
	tests := []struct {
		name            string
		interactionMode string
		driveName       string
		wantType        mind.ActionType
		wantExecutor    string
	}{
		{
			name:            "family bridge waits during conversation",
			interactionMode: "conversation",
			driveName:       mind.DriveFamilyBridge,
			wantType:        mind.ActionWait,
			wantExecutor:    "mind",
		},
		{
			name:            "family bridge speaks message when idle",
			interactionMode: "idle",
			driveName:       mind.DriveFamilyBridge,
			wantType:        mind.ActionSpeak,
			wantExecutor:    "message",
		},
		{
			name:         "companionship speaks greeting",
			driveName:    mind.DriveCompanionship,
			wantType:     mind.ActionSpeak,
			wantExecutor: "greeting",
		},
		{
			name:         "unknown drive updates state silently",
			driveName:    "unknown",
			wantType:     mind.ActionSilentStateUpdate,
			wantExecutor: "mind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(
				mind.Situation{DeviceID: "dev-001", InteractionMode: tt.interactionMode},
				mind.SelfState{},
				[]mind.Thought{{
					ID:        "thought-1",
					DeviceID:  "dev-001",
					DriveName: tt.driveName,
				}},
			)
			if len(got) != 1 {
				t.Fatalf("actions = %+v, want 1", got)
			}
			if got[0].Type != tt.wantType || got[0].Executor != tt.wantExecutor {
				t.Fatalf("action = %+v, want %s/%s", got[0], tt.wantType, tt.wantExecutor)
			}
		})
	}
}

func TestSelectClampsScoreEdges(t *testing.T) {
	tests := []struct {
		name    string
		thought mind.Thought
		want    float64
	}{
		{
			name: "nan becomes zero",
			thought: mind.Thought{
				ID:        "thought-nan",
				DriveName: mind.DriveStewardship,
				Urgency:   math.NaN(),
			},
			want: 0,
		},
		{
			name: "negative becomes zero",
			thought: mind.Thought{
				ID:               "thought-negative",
				DriveName:        mind.DriveStewardship,
				InterruptionCost: 10,
			},
			want: 0,
		},
		{
			name: "overflow becomes one",
			thought: mind.Thought{
				ID:        "thought-overflow",
				DriveName: mind.DriveStewardship,
				Urgency:   math.Inf(1),
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(
				mind.Situation{DeviceID: "dev-001", InteractionMode: "idle"},
				mind.SelfState{},
				[]mind.Thought{tt.thought},
			)
			if len(got) != 1 {
				t.Fatalf("actions = %+v, want 1", got)
			}
			if got[0].Score != tt.want {
				t.Fatalf("Score = %v, want %v", got[0].Score, tt.want)
			}
		})
	}
}
