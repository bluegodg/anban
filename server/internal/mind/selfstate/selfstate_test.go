package selfstate

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestDefaultStateUsesApprovedPersonaWeights(t *testing.T) {
	state := Default("dev-001", time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC))
	if state.FamilyWeight != 0.60 || state.PetWeight != 0.25 || state.StewardWeight != 0.15 {
		t.Fatalf("weights = family %.2f pet %.2f steward %.2f", state.FamilyWeight, state.PetWeight, state.StewardWeight)
	}
	if state.Warmth <= 0 || state.Patience <= 0 {
		t.Fatalf("state = %+v, want positive warmth and patience", state)
	}
}

func TestApplyEventsAdjustsConcernAndPlayfulness(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(time.Minute)},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at.Add(2 * time.Minute)},
	})
	if updated.Concern <= state.Concern {
		t.Fatalf("Concern = %.2f, want greater than %.2f", updated.Concern, state.Concern)
	}
	if updated.Curiosity <= state.Curiosity {
		t.Fatalf("Curiosity = %.2f, want greater than %.2f", updated.Curiosity, state.Curiosity)
	}
}

func TestApplyEventsAdvancesAtToNewestEventAndPreservesDeviceID(t *testing.T) {
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	newest := at.Add(2 * time.Hour)

	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at.Add(time.Hour)},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventLongSilence, At: newest},
		{ID: "evt-3", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at.Add(30 * time.Minute)},
	})

	if updated.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want preserved dev-001", updated.DeviceID)
	}
	if !updated.At.Equal(newest) {
		t.Fatalf("At = %v, want newest event time %v", updated.At, newest)
	}
}

func TestApplyEventsClampsValuesAndLearnsFromRejectedFeedback(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	state.Concern = 0.97
	state.Quietness = 0.96
	state.Playfulness = 0.02

	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(time.Minute)},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventFeedbackObserved, At: at.Add(2 * time.Minute), Emotion: "rejected"},
	})

	if updated.Concern != 1 {
		t.Fatalf("Concern = %.2f, want clamped to 1", updated.Concern)
	}
	if updated.Quietness != 1 {
		t.Fatalf("Quietness = %.2f, want clamped to 1", updated.Quietness)
	}
	if updated.Playfulness != 0 {
		t.Fatalf("Playfulness = %.2f, want clamped to 0 after rejected feedback", updated.Playfulness)
	}
}

func TestApplyEventsSortsRecentFirstEventsWithoutMutatingCallerSlice(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	state.Concern = 0.95
	events := []mind.Event{
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventReminderAcknowledged, At: at.Add(2 * time.Minute)},
		{ID: "evt-old", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(time.Minute)},
	}

	updated := ApplyEvents(state, events)

	if math.Abs(updated.Concern-0.95) > 1e-9 {
		t.Fatalf("Concern = %.2f, want old silence clamped then newer acknowledgement to leave 0.95", updated.Concern)
	}
	if !slices.Equal([]string{events[0].ID, events[1].ID}, []string{"evt-new", "evt-old"}) {
		t.Fatalf("events mutated to %+v, want caller order preserved", events)
	}
}

func TestApplyEventsSameTimestampTreatsEarlierRecentFirstEventAsNewer(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	state.Concern = 0.95
	sameTime := at.Add(time.Minute)

	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventReminderAcknowledged, At: sameTime},
		{ID: "evt-old", DeviceID: "dev-001", Type: mind.EventLongSilence, At: sameTime},
	})

	if math.Abs(updated.Concern-0.95) > 1e-9 {
		t.Fatalf("Concern = %.2f, want same-time recent-first acknowledgement to apply last", updated.Concern)
	}
}

func TestApplyEventsIgnoresZeroTimeAndStaleEvents(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-zero", DeviceID: "dev-001", Type: mind.EventLongSilence},
		{ID: "evt-old", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-time.Minute)},
		{ID: "evt-equal", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at},
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at.Add(time.Minute)},
	})

	if updated.Concern != state.Concern {
		t.Fatalf("Concern = %.2f, want stale long silence ignored at %.2f", updated.Concern, state.Concern)
	}
	if updated.Curiosity != state.Curiosity {
		t.Fatalf("Curiosity = %.2f, want equal-time presence ignored at %.2f", updated.Curiosity, state.Curiosity)
	}
	if math.Abs(updated.Warmth-(state.Warmth+0.04)) > 1e-9 {
		t.Fatalf("Warmth = %.2f, want only newer child message to apply", updated.Warmth)
	}
	if !updated.At.Equal(at.Add(time.Minute)) {
		t.Fatalf("At = %v, want only newer event time %v", updated.At, at.Add(time.Minute))
	}
}
