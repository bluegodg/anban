package thoughts

import (
	"fmt"
	"math"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Generate(s mind.Situation, state mind.SelfState, drives []mind.Drive, events []mind.Event) []mind.Thought {
	out := []mind.Thought{}
	usedSourceIDs := make(map[string]map[string]bool)
	for i, event := range events {
		switch event.Type {
		case mind.EventLongSilence:
			sourceID, deviceID, at := normalizedEventIdentity(s, event, i, "long-silence", usedSourceIDs)
			driveName := mind.DriveCare
			if driveStrength(drives, mind.DriveQuietPresence) >= driveStrength(drives, mind.DriveCare) {
				driveName = mind.DriveQuietPresence
			}
			out = append(out, clampThought(mind.Thought{
				ID:               fmt.Sprintf("thought-%s-long-silence", sourceID),
				DeviceID:         deviceID,
				At:               at,
				Content:          "老人安静了一段时间，先判断是否适合轻声关心，或者只安静陪着。",
				SourceEventIDs:   []string{sourceID},
				DriveName:        driveName,
				EmotionalTone:    "quiet",
				Urgency:          0.35 + state.Concern*0.2,
				CareValue:        0.55 + state.Concern*0.25,
				Novelty:          0.2,
				InterruptionCost: 0.55 + state.Quietness*0.25,
				Intimacy:         0.4 + state.Warmth*0.2,
				Status:           mind.ThoughtPending,
			}))
		case mind.EventChildMessageReceived:
			sourceID, deviceID, at := normalizedEventIdentity(s, event, i, "child-message", usedSourceIDs)
			out = append(out, clampThought(mind.Thought{
				ID:               fmt.Sprintf("thought-%s-child-message", sourceID),
				DeviceID:         deviceID,
				At:               at,
				Content:          "子女发来了消息，需要找一个不打扰的时机带给老人。",
				SourceEventIDs:   []string{sourceID},
				DriveName:        mind.DriveFamilyBridge,
				EmotionalTone:    "warm",
				Urgency:          0.55,
				CareValue:        0.78,
				Novelty:          0.45,
				InterruptionCost: 0.35,
				Intimacy:         0.7,
				Status:           mind.ThoughtPending,
			}))
		case mind.EventReminderDue:
			sourceID, deviceID, at := normalizedEventIdentity(s, event, i, "reminder", usedSourceIDs)
			out = append(out, clampThought(mind.Thought{
				ID:               fmt.Sprintf("thought-%s-reminder", sourceID),
				DeviceID:         deviceID,
				At:               at,
				Content:          "提醒到期了，应该用家人式语气轻轻带到，而不是像命令。",
				SourceEventIDs:   []string{sourceID},
				DriveName:        mind.DriveStewardship,
				EmotionalTone:    "caring",
				Urgency:          0.75,
				CareValue:        0.72,
				Novelty:          0.15,
				InterruptionCost: 0.30,
				Intimacy:         0.55,
				Status:           mind.ThoughtPending,
			}))
		case mind.EventPresenceSeen:
			sourceID, deviceID, at := normalizedEventIdentity(s, event, i, "presence", usedSourceIDs)
			out = append(out, clampThought(mind.Thought{
				ID:               fmt.Sprintf("thought-%s-presence", sourceID),
				DeviceID:         deviceID,
				At:               at,
				Content:          "老人出现了，可以轻轻保持存在感，也可以先观察不打扰。",
				SourceEventIDs:   []string{sourceID},
				DriveName:        mind.DriveCompanionship,
				EmotionalTone:    "warm",
				Urgency:          0.35,
				CareValue:        0.45,
				Novelty:          0.35,
				InterruptionCost: 0.40,
				Intimacy:         0.55,
				Status:           mind.ThoughtPending,
			}))
		}
	}
	return out
}

func normalizedEventIdentity(s mind.Situation, event mind.Event, index int, thoughtKind string, used map[string]map[string]bool) (string, string, time.Time) {
	deviceID := event.DeviceID
	if deviceID == "" {
		deviceID = s.DeviceID
	}
	at := event.At
	if at.IsZero() {
		at = s.At
	}

	sourceID := event.ID
	if sourceID == "" || sourceIDUsed(used, thoughtKind, sourceID) {
		sourceID = fallbackSourceID(deviceID, event.Type, at, index)
		for attempt := 1; sourceIDUsed(used, thoughtKind, sourceID); attempt++ {
			sourceID = fmt.Sprintf("%s-%d", fallbackSourceID(deviceID, event.Type, at, index), attempt)
		}
	}
	markSourceIDUsed(used, thoughtKind, sourceID)
	return sourceID, deviceID, at
}

func fallbackSourceID(deviceID string, eventType mind.EventType, at time.Time, index int) string {
	return fmt.Sprintf("generated-%s-%s-%s-%d", deviceID, eventType, at.UTC().Format("20060102T150405.000000000Z"), index)
}

func sourceIDUsed(used map[string]map[string]bool, thoughtKind string, sourceID string) bool {
	return used[thoughtKind] != nil && used[thoughtKind][sourceID]
}

func markSourceIDUsed(used map[string]map[string]bool, thoughtKind string, sourceID string) {
	if used[thoughtKind] == nil {
		used[thoughtKind] = make(map[string]bool)
	}
	used[thoughtKind][sourceID] = true
}

func clampThought(thought mind.Thought) mind.Thought {
	thought.Urgency = clampScore(thought.Urgency)
	thought.CareValue = clampScore(thought.CareValue)
	thought.Novelty = clampScore(thought.Novelty)
	thought.InterruptionCost = clampScore(thought.InterruptionCost)
	thought.Intimacy = clampScore(thought.Intimacy)
	return thought
}

func clampScore(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func driveStrength(drives []mind.Drive, name string) float64 {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.Strength
		}
	}
	return 0
}
