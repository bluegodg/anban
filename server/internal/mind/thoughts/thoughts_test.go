package thoughts

import (
	"math"
	"reflect"
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

func TestGenerateNormalizesMissingEventIdentityFromSituation(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	input := []mind.Event{{Type: mind.EventReminderDue}}
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at},
		mind.SelfState{},
		nil,
		input,
	)
	gotAgain := Generate(
		mind.Situation{DeviceID: "dev-001", At: at},
		mind.SelfState{},
		nil,
		input,
	)
	if len(got) != 1 {
		t.Fatalf("thoughts = %+v, want 1", got)
	}
	thought := got[0]
	wantSource := "generated-dev-001-reminder_due-20260616T093000.000000000Z-0"
	wantID := "thought-generated-dev-001-reminder_due-20260616T093000.000000000Z-0-reminder"
	if thought.ID != wantID {
		t.Fatalf("ID = %q, want %q", thought.ID, wantID)
	}
	if !reflect.DeepEqual(thought.SourceEventIDs, []string{wantSource}) {
		t.Fatalf("SourceEventIDs = %+v, want [%q]", thought.SourceEventIDs, wantSource)
	}
	if len(gotAgain) != 1 {
		t.Fatalf("second thoughts = %+v, want 1", gotAgain)
	}
	if gotAgain[0].ID != thought.ID {
		t.Fatalf("second ID = %q, want deterministic ID %q", gotAgain[0].ID, thought.ID)
	}
	if !reflect.DeepEqual(gotAgain[0].SourceEventIDs, thought.SourceEventIDs) {
		t.Fatalf("second SourceEventIDs = %+v, want deterministic source IDs %+v", gotAgain[0].SourceEventIDs, thought.SourceEventIDs)
	}
	if thought.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want situation device", thought.DeviceID)
	}
	if !thought.At.Equal(at) {
		t.Fatalf("At = %v, want situation time %v", thought.At, at)
	}
}

func TestGenerateDuplicateEventIDsOfSameTypeGetUniqueThoughtIDs(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at},
		mind.SelfState{},
		nil,
		[]mind.Event{
			{ID: "evt-1", Type: mind.EventReminderDue, DeviceID: "dev-001", At: at},
			{ID: "evt-1", Type: mind.EventReminderDue, DeviceID: "dev-001", At: at.Add(time.Minute)},
		},
	)
	if len(got) != 2 {
		t.Fatalf("thoughts = %+v, want 2", got)
	}
	if got[0].ID != "thought-evt-1-reminder" {
		t.Fatalf("first ID = %q, want raw event ID for first occurrence", got[0].ID)
	}
	if got[0].SourceEventIDs[0] != "evt-1" {
		t.Fatalf("first SourceEventIDs = %+v, want raw event ID", got[0].SourceEventIDs)
	}
	if got[1].ID == got[0].ID {
		t.Fatalf("duplicate thought IDs: %+v", got)
	}
	if got[1].SourceEventIDs[0] != "evt-1" {
		t.Fatalf("second SourceEventIDs = %+v, want raw duplicate event ID", got[1].SourceEventIDs)
	}
}

func TestGenerateEmptyEventIDsUseIndexInFallbackIdentity(t *testing.T) {
	at := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at.Add(time.Hour)},
		mind.SelfState{},
		nil,
		[]mind.Event{
			{DeviceID: "dev-event", Type: mind.EventReminderDue, At: at},
			{DeviceID: "dev-event", Type: mind.EventReminderDue, At: at},
		},
	)
	if len(got) != 2 {
		t.Fatalf("thoughts = %+v, want 2", got)
	}

	wantSources := []string{
		"generated-dev-event-reminder_due-20260616T093000.000000000Z-0",
		"generated-dev-event-reminder_due-20260616T093000.000000000Z-1",
	}
	wantIDs := []string{
		"thought-generated-dev-event-reminder_due-20260616T093000.000000000Z-0-reminder",
		"thought-generated-dev-event-reminder_due-20260616T093000.000000000Z-1-reminder",
	}
	for i, thought := range got {
		if thought.ID != wantIDs[i] {
			t.Fatalf("thought[%d].ID = %q, want %q", i, thought.ID, wantIDs[i])
		}
		if !reflect.DeepEqual(thought.SourceEventIDs, []string{wantSources[i]}) {
			t.Fatalf("thought[%d].SourceEventIDs = %+v, want [%q]", i, thought.SourceEventIDs, wantSources[i])
		}
	}
}

func TestGenerateClampsScoresFromOutOfRangeSelfState(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at},
		mind.SelfState{Concern: 10, Quietness: 10, Warmth: -10},
		[]mind.Drive{{Name: mind.DriveCare, Strength: 0.6}},
		[]mind.Event{{ID: "evt-1", Type: mind.EventLongSilence, DeviceID: "dev-001", At: at}},
	)
	if len(got) != 1 {
		t.Fatalf("thoughts = %+v, want 1", got)
	}
	thought := got[0]
	assertScore(t, "Urgency", thought.Urgency, 1)
	assertScore(t, "CareValue", thought.CareValue, 1)
	assertScore(t, "Novelty", thought.Novelty, 0.2)
	assertScore(t, "InterruptionCost", thought.InterruptionCost, 1)
	assertScore(t, "Intimacy", thought.Intimacy, 0)
}

func TestGenerateClampsNaNScores(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Generate(
		mind.Situation{DeviceID: "dev-001", At: at},
		mind.SelfState{Concern: math.NaN(), Quietness: 0.5, Warmth: 0.5},
		[]mind.Drive{{Name: mind.DriveCare, Strength: 0.6}},
		[]mind.Event{{ID: "evt-1", Type: mind.EventLongSilence, DeviceID: "dev-001", At: at}},
	)
	if len(got) != 1 {
		t.Fatalf("thoughts = %+v, want 1", got)
	}
	assertScore(t, "Urgency", got[0].Urgency, 0)
	assertScore(t, "CareValue", got[0].CareValue, 0)
}

func TestGenerateReminderDueAndPresenceSeenThoughtFields(t *testing.T) {
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name             string
		event            mind.Event
		wantID           string
		wantSources      []string
		wantDrive        string
		wantUrgency      float64
		wantCare         float64
		wantNovelty      float64
		wantInterruption float64
		wantIntimacy     float64
	}{
		{
			name:             "reminder due",
			event:            mind.Event{ID: "rem-1", DeviceID: "dev-event", Type: mind.EventReminderDue, At: at},
			wantID:           "thought-rem-1-reminder",
			wantSources:      []string{"rem-1"},
			wantDrive:        mind.DriveStewardship,
			wantUrgency:      0.75,
			wantCare:         0.72,
			wantNovelty:      0.15,
			wantInterruption: 0.30,
			wantIntimacy:     0.55,
		},
		{
			name:             "presence seen",
			event:            mind.Event{ID: "presence-1", DeviceID: "dev-event", Type: mind.EventPresenceSeen, At: at},
			wantID:           "thought-presence-1-presence",
			wantSources:      []string{"presence-1"},
			wantDrive:        mind.DriveCompanionship,
			wantUrgency:      0.35,
			wantCare:         0.45,
			wantNovelty:      0.35,
			wantInterruption: 0.40,
			wantIntimacy:     0.55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate(
				mind.Situation{DeviceID: "dev-situation", At: at.Add(time.Hour)},
				mind.SelfState{},
				nil,
				[]mind.Event{tt.event},
			)
			if len(got) != 1 {
				t.Fatalf("thoughts = %+v, want 1", got)
			}
			thought := got[0]
			if thought.ID != tt.wantID {
				t.Fatalf("ID = %q, want %q", thought.ID, tt.wantID)
			}
			if !reflect.DeepEqual(thought.SourceEventIDs, tt.wantSources) {
				t.Fatalf("SourceEventIDs = %+v, want %+v", thought.SourceEventIDs, tt.wantSources)
			}
			if thought.DeviceID != "dev-event" {
				t.Fatalf("DeviceID = %q, want event device", thought.DeviceID)
			}
			if !thought.At.Equal(at) {
				t.Fatalf("At = %v, want event time %v", thought.At, at)
			}
			if thought.Status != mind.ThoughtPending {
				t.Fatalf("Status = %q, want pending", thought.Status)
			}
			if thought.DriveName != tt.wantDrive {
				t.Fatalf("DriveName = %q, want %q", thought.DriveName, tt.wantDrive)
			}
			assertScore(t, "Urgency", thought.Urgency, tt.wantUrgency)
			assertScore(t, "CareValue", thought.CareValue, tt.wantCare)
			assertScore(t, "Novelty", thought.Novelty, tt.wantNovelty)
			assertScore(t, "InterruptionCost", thought.InterruptionCost, tt.wantInterruption)
			assertScore(t, "Intimacy", thought.Intimacy, tt.wantIntimacy)
		})
	}
}

func assertScore(t *testing.T, name string, got float64, want float64) {
	t.Helper()
	if math.IsNaN(got) || got < 0 || got > 1 {
		t.Fatalf("%s = %v, want clamped score in [0,1]", name, got)
	}
	if got != want {
		t.Fatalf("%s = %.2f, want %.2f", name, got, want)
	}
}
