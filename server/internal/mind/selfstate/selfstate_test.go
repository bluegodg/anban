package selfstate

import (
	"fmt"
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
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at},
	})
	if updated.Concern <= state.Concern {
		t.Fatalf("Concern = %.2f, want greater than %.2f", updated.Concern, state.Concern)
	}
	if updated.Curiosity <= state.Curiosity {
		t.Fatalf("Curiosity = %.2f, want greater than %.2f", updated.Curiosity, state.Curiosity)
	}
}

func TestApplyEventsConversationChangesPresenceState(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-user", DeviceID: "dev-001", Type: mind.EventElderSpoke, At: at.Add(time.Minute)},
		{ID: "evt-assistant", DeviceID: "dev-001", Type: mind.EventAssistantSpoke, At: at.Add(2 * time.Minute)},
	})

	if updated.Warmth <= state.Warmth {
		t.Fatalf("Warmth = %.2f, want greater than %.2f after conversation", updated.Warmth, state.Warmth)
	}
	if updated.Energy <= state.Energy {
		t.Fatalf("Energy = %.2f, want greater than %.2f after elder spoke", updated.Energy, state.Energy)
	}
	if updated.Quietness >= state.Quietness {
		t.Fatalf("Quietness = %.2f, want lower than %.2f after elder spoke", updated.Quietness, state.Quietness)
	}
	if updated.Confidence <= state.Confidence {
		t.Fatalf("Confidence = %.2f, want greater than %.2f after assistant response", updated.Confidence, state.Confidence)
	}
}

func TestApplyEventsSkipsProcessedEventIDs(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	event := mind.Event{ID: "evt-duplicate", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at}

	first := ApplyEvents(state, []mind.Event{event})
	second := ApplyEvents(first, []mind.Event{event})

	if second.Concern != first.Concern {
		t.Fatalf("Concern = %.2f, want unchanged at %.2f for processed event", second.Concern, first.Concern)
	}
	if !slices.Equal(second.ProcessedEventIDs, []string{"evt-duplicate"}) {
		t.Fatalf("ProcessedEventIDs = %+v, want duplicate event recorded once", second.ProcessedEventIDs)
	}
}

func TestApplyEventsIgnoresEmptyIDEvent(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)

	updated := ApplyEvents(state, []mind.Event{
		{DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(time.Minute)},
	})

	if updated.Concern != state.Concern {
		t.Fatalf("Concern = %.2f, want empty-ID event ignored at %.2f", updated.Concern, state.Concern)
	}
	if !updated.At.Equal(state.At) {
		t.Fatalf("At = %v, want unchanged at %v for empty-ID event", updated.At, state.At)
	}
	if len(updated.ProcessedEventIDs) != 0 {
		t.Fatalf("ProcessedEventIDs = %+v, want no empty ID recorded", updated.ProcessedEventIDs)
	}
}

func TestApplyEventsAppliesEqualTimeDifferentIDsWhenUnprocessed(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	state.ProcessedEventIDs = []string{"evt-already"}

	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at},
	})

	if updated.Concern <= state.Concern {
		t.Fatalf("Concern = %.2f, want equal-time unprocessed event to apply over %.2f", updated.Concern, state.Concern)
	}
	if !slices.Equal(updated.ProcessedEventIDs, []string{"evt-already", "evt-new"}) {
		t.Fatalf("ProcessedEventIDs = %+v, want existing and new event IDs", updated.ProcessedEventIDs)
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

func TestApplyEventsNormalizesAndBoundsProcessedEventIDs(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	state.ProcessedEventIDs = []string{"", "old-1", "old-2", "old-1", "", "old-3", "old-2"}

	events := make([]mind.Event, 132)
	for i := range events {
		events[i] = mind.Event{
			ID:       fmt.Sprintf("evt-%03d", i+1),
			DeviceID: "dev-001",
			Type:     mind.EventChildMessageReceived,
			At:       at.Add(time.Duration(i+1) * time.Minute),
		}
	}

	updated := ApplyEvents(state, events)

	if len(updated.ProcessedEventIDs) > maxProcessedEventIDs {
		t.Fatalf("ProcessedEventIDs len = %d, want <= %d", len(updated.ProcessedEventIDs), maxProcessedEventIDs)
	}

	seen := make(map[string]struct{}, len(updated.ProcessedEventIDs))
	for _, id := range updated.ProcessedEventIDs {
		if id == "" {
			t.Fatalf("ProcessedEventIDs = %+v, want no empty IDs", updated.ProcessedEventIDs)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("ProcessedEventIDs = %+v, want no duplicate IDs", updated.ProcessedEventIDs)
		}
		seen[id] = struct{}{}
	}

	for _, id := range []string{"evt-130", "evt-131", "evt-132"} {
		if _, ok := seen[id]; !ok {
			t.Fatalf("ProcessedEventIDs = %+v, want newest ID %s retained", updated.ProcessedEventIDs, id)
		}
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

func TestApplyEventsIgnoresZeroTimeEvent(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-zero", DeviceID: "dev-001", Type: mind.EventLongSilence},
		{ID: "evt-equal", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at},
		{ID: "evt-new", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at.Add(time.Minute)},
	})

	if updated.Concern != state.Concern {
		t.Fatalf("Concern = %.2f, want zero-time long silence ignored at %.2f", updated.Concern, state.Concern)
	}
	if updated.Curiosity <= state.Curiosity {
		t.Fatalf("Curiosity = %.2f, want equal-time presence to apply over %.2f", updated.Curiosity, state.Curiosity)
	}
	if math.Abs(updated.Warmth-(state.Warmth+0.06)) > 1e-9 {
		t.Fatalf("Warmth = %.2f, want equal-time presence and newer child message to apply", updated.Warmth)
	}
	if !updated.At.Equal(at.Add(time.Minute)) {
		t.Fatalf("At = %v, want only newer event time %v", updated.At, at.Add(time.Minute))
	}
}
