package selfstate

import (
	"math"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Default(deviceID string, at time.Time) mind.SelfState {
	return mind.SelfState{
		DeviceID:      deviceID,
		At:            at,
		Warmth:        0.55,
		Concern:       0.30,
		Curiosity:     0.35,
		Playfulness:   0.25,
		Energy:        0.50,
		Quietness:     0.45,
		Patience:      0.70,
		Confidence:    0.60,
		FamilyWeight:  0.60,
		PetWeight:     0.25,
		StewardWeight: 0.15,
	}
}

func ApplyEvents(state mind.SelfState, events []mind.Event) mind.SelfState {
	for _, event := range events {
		if event.At.After(state.At) {
			state.At = event.At
		}
		switch event.Type {
		case mind.EventLongSilence:
			state.Concern = clamp(state.Concern + 0.08)
			state.Quietness = clamp(state.Quietness + 0.04)
		case mind.EventPresenceSeen:
			state.Curiosity = clamp(state.Curiosity + 0.05)
			state.Warmth = clamp(state.Warmth + 0.02)
		case mind.EventChildMessageReceived:
			state.Warmth = clamp(state.Warmth + 0.04)
		case mind.EventReminderAcknowledged:
			state.Concern = clamp(state.Concern - 0.05)
			state.Warmth = clamp(state.Warmth + 0.02)
		case mind.EventFeedbackObserved:
			if event.Emotion == "rejected" {
				state.Quietness = clamp(state.Quietness + 0.08)
				state.Playfulness = clamp(state.Playfulness - 0.04)
			}
		}
	}
	return state
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
