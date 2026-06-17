package situation

import (
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Build(deviceID string, at time.Time, events []mind.Event) mind.Situation {
	out := mind.Situation{
		DeviceID:        deviceID,
		At:              at,
		TimeOfDay:       timeOfDay(at),
		ElderPresence:   "unknown",
		InteractionMode: "idle",
		ActivityLevel:   "normal",
		EmotionalTone:   "uncertain",
		SocialContext:   "alone",
	}
	for _, event := range events {
		switch event.Type {
		case mind.EventPresenceSeen:
			out.ElderPresence = "present"
			out.ActivityLevel = "normal"
		case mind.EventPresenceAbsent:
			out.ElderPresence = "absent"
		case mind.EventLongSilence:
			out.InteractionMode = "idle"
			out.ActivityLevel = "low"
			out.EmotionalTone = "quiet"
			out.Constraints = addUnique(out.Constraints, "avoid_long_speech")
			out.Constraints = addUnique(out.Constraints, "prefer_observation")
		case mind.EventChildMessageReceived:
			out.SocialContext = "child_waiting_reply"
			out.OpenLoops = addUnique(out.OpenLoops, "child_message_pending")
		case mind.EventReminderDue:
			out.OpenLoops = addUnique(out.OpenLoops, "reminder_due")
		case mind.EventElderSpoke:
			out.ElderPresence = "present"
			out.InteractionMode = "conversation"
			out.ActivityLevel = "normal"
		}
	}
	if out.TimeOfDay == "night" {
		out.Constraints = addUnique(out.Constraints, "prefer_short_phrase")
	}
	return out
}

func timeOfDay(at time.Time) string {
	switch hour := at.Hour(); {
	case hour >= 5 && hour <= 10:
		return "morning"
	case hour >= 11 && hour <= 13:
		return "noon"
	case hour >= 14 && hour <= 17:
		return "afternoon"
	case hour >= 18 && hour <= 21:
		return "evening"
	default:
		return "night"
	}
}

func addUnique(values []string, next string) []string {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
