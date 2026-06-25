package behavior

import (
	"math"
	"strings"
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
		mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "night", Constraints: []string{"prefer_observation", mind.ConstraintMindProactiveDaytimeOnly}},
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

func TestSelectTurnsHighConcernQuietPresenceIntoGreetingSpeak(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	got := Select(
		mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "morning", InteractionMode: "idle"},
		mind.SelfState{Concern: 0.82, Warmth: 0.55, FamilyWeight: 0.6, PetWeight: 0.25, StewardWeight: 0.15},
		[]mind.Thought{{
			ID:               "thought-silence",
			DeviceID:         "dev-001",
			At:               at,
			DriveName:        mind.DriveQuietPresence,
			Content:          "老人安静了一段时间",
			Urgency:          0.52,
			CareValue:        0.76,
			InterruptionCost: 0.55,
			Intimacy:         0.55,
		}},
	)
	if len(got) != 1 {
		t.Fatalf("actions = %+v, want 1", got)
	}
	action := got[0]
	if action.Type != mind.ActionSpeak || action.Executor != "greeting" {
		t.Fatalf("action = %+v, want greeting speak", action)
	}
	if action.Text == "" || action.Text == "我在呢。" {
		t.Fatalf("Text = %q, want deterministic gentle check-in template", action.Text)
	}
	if action.Args["mindProactive"] != true {
		t.Fatalf("Args = %+v, want mindProactive=true marker", action.Args)
	}
	if action.Score < 0.35 {
		t.Fatalf("Score = %.2f, want high enough for expression gate", action.Score)
	}
}

func TestSelectKeepsHighInterruptionVisionFollowUpQuietEvenWhenConcernHigh(t *testing.T) {
	actions := Select(
		mind.Situation{DeviceID: "dev-001", TimeOfDay: "morning", InteractionMode: "idle"},
		mind.SelfState{Concern: 1, Quietness: 1},
		[]mind.Thought{{
			ID:               "thought-vision-observation",
			DeviceID:         "dev-001",
			DriveName:        mind.DriveQuietPresence,
			Content:          "画面中没有出现老人",
			CareValue:        0.65,
			InterruptionCost: 0.85,
		}},
	)
	if len(actions) != 1 || actions[0].Type != mind.ActionWait {
		t.Fatalf("actions = %+v, want quiet wait after high-cost vision observation", actions)
	}
}

func TestSelectKeepsQuietPresenceWaitingWhenConcernLowOrCoolingDown(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	thought := mind.Thought{
		ID:               "thought-silence",
		DeviceID:         "dev-001",
		At:               at,
		DriveName:        mind.DriveQuietPresence,
		Content:          "老人安静了一段时间",
		Urgency:          0.40,
		CareValue:        0.50, // below the loosened 0.55 care bar, so low-concern still waits
		InterruptionCost: 0.60,
		Intimacy:         0.50,
	}

	tests := []struct {
		name        string
		situation   mind.Situation
		state       mind.SelfState
		wantReason  string
		wantNoSpeak bool
	}{
		{
			name:      "low concern waits",
			situation: mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "morning", InteractionMode: "idle"},
			state:     mind.SelfState{Concern: 0.35, Quietness: 0.60},
		},
		{
			name: "cooldown waits even with high concern",
			situation: mind.Situation{
				DeviceID:        "dev-001",
				At:              at,
				TimeOfDay:       "morning",
				InteractionMode: "idle",
				Constraints:     []string{mind.ConstraintMindProactiveCooldownActive},
			},
			state:      mind.SelfState{Concern: 0.85, Quietness: 0.30},
			wantReason: "冷却",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Select(tt.situation, tt.state, []mind.Thought{thought})
			if len(got) != 1 {
				t.Fatalf("actions = %+v, want 1", got)
			}
			if got[0].Type != mind.ActionWait || got[0].Executor != "mind" {
				t.Fatalf("action = %+v, want wait/mind", got[0])
			}
			if tt.wantReason != "" && !strings.Contains(got[0].Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want contain %q", got[0].Reason, tt.wantReason)
			}
		})
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

func TestSelectTurnsHighCareThoughtIntoAutonomousVisionAction(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	actions := Select(
		mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "morning", InteractionMode: "idle"},
		mind.SelfState{Concern: 0.82, Curiosity: 0.62},
		[]mind.Thought{{
			ID:               "thought-care",
			DeviceID:         "dev-001",
			At:               at,
			DriveName:        mind.DriveCare,
			Content:          "老人安静了一段时间",
			Urgency:          0.52,
			CareValue:        0.78,
			InterruptionCost: 0.45,
		}},
	)
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	action := actions[0]
	if action.Type != mind.ActionCallMCPTool || action.Executor != "vision" {
		t.Fatalf("action = %+v, want vision MCP action", action)
	}
	if action.Args["mindAutonomousVision"] != true || action.Args["tool"] != "self.camera.take_photo" {
		t.Fatalf("Args = %+v, want autonomous vision marker and camera tool", action.Args)
	}
	if action.Score < 0.20 {
		t.Fatalf("Score = %.2f, want expression gate to keep the action pending", action.Score)
	}
}

func TestSelectDefersAutonomousVisionWhenGuarded(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	thought := mind.Thought{
		ID:               "thought-care",
		DeviceID:         "dev-001",
		At:               at,
		DriveName:        mind.DriveCare,
		CareValue:        0.78,
		InterruptionCost: 0.45,
	}
	tests := []struct {
		name        string
		constraints []string
		state       mind.SelfState
		careValue   float64
		wantReason  string
	}{
		{name: "disabled", constraints: []string{"mind_autonomous_vision_disabled"}, state: mind.SelfState{Concern: 0.82}, careValue: 0.78, wantReason: "关闭"},
		{name: "night", constraints: []string{mind.ConstraintMindProactiveDaytimeOnly}, state: mind.SelfState{Concern: 0.82}, careValue: 0.78, wantReason: "白天"},
		{name: "cooldown", constraints: []string{"mind_autonomous_vision_cooldown_active"}, state: mind.SelfState{Concern: 0.82}, careValue: 0.78, wantReason: "冷却"},
		{name: "low concern", state: mind.SelfState{Concern: 0.30}, careValue: 0.60, wantReason: "关心"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guardedThought := thought
			guardedThought.CareValue = tt.careValue
			actions := Select(
				mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "morning", InteractionMode: "idle", Constraints: tt.constraints},
				tt.state,
				[]mind.Thought{guardedThought},
			)
			if len(actions) != 1 || actions[0].Type != mind.ActionScheduleRecheck {
				t.Fatalf("actions = %+v, want schedule recheck", actions)
			}
			if !strings.Contains(actions[0].Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want contain %q", actions[0].Reason, tt.wantReason)
			}
		})
	}
}

func TestSelectExplainsVisionCooldownWhenHighInterruptionCareWaits(t *testing.T) {
	actions := Select(
		mind.Situation{
			DeviceID:        "dev-001",
			TimeOfDay:       "morning",
			InteractionMode: "idle",
			Constraints:     []string{"mind_autonomous_vision_cooldown_active"},
		},
		mind.SelfState{Concern: 1, Quietness: 1},
		[]mind.Thought{{
			ID:               "thought-care",
			DeviceID:         "dev-001",
			DriveName:        mind.DriveCare,
			CareValue:        0.8,
			InterruptionCost: 0.8,
		}},
	)
	if len(actions) != 1 || actions[0].Type != mind.ActionWait {
		t.Fatalf("actions = %+v, want wait", actions)
	}
	if !strings.Contains(actions[0].Reason, "冷却") {
		t.Fatalf("Reason = %q, want explicit cooldown", actions[0].Reason)
	}
}
