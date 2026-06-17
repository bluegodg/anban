package situation

import (
	"sort"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Build(deviceID string, at time.Time, events []mind.Event) mind.Situation {
	return BuildWithLocation(deviceID, at, events, at.Location())
}

func BuildWithLocation(deviceID string, at time.Time, events []mind.Event, loc *time.Location) mind.Situation {
	localAt := at
	if loc != nil {
		localAt = at.In(loc)
	}
	out := mind.Situation{
		DeviceID:        deviceID,
		At:              at,
		TimeOfDay:       timeOfDay(localAt),
		ElderPresence:   "unknown",
		InteractionMode: "idle",
		ActivityLevel:   "normal",
		EmotionalTone:   "uncertain",
		SocialContext:   "alone",
	}
	ordered := make([]orderedEvent, len(events))
	for i, event := range events {
		ordered[i] = orderedEvent{event: event, index: i}
	}
	sort.Slice(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		// Recent windows arrive newest-first; descending index makes equal timestamps apply older before newer.
		if left.event.At.Equal(right.event.At) {
			return left.index > right.index
		}
		return left.event.At.Before(right.event.At)
	})
	for _, item := range ordered {
		event := item.event
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

type orderedEvent struct {
	event mind.Event
	index int
}

// timeOfDay uses at's own location and wall clock; callers must pass device-local time.
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
