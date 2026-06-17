package selfstate

import (
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
