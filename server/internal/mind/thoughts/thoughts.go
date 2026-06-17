package thoughts

import (
	"fmt"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Generate(s mind.Situation, state mind.SelfState, drives []mind.Drive, events []mind.Event) []mind.Thought {
	out := []mind.Thought{}
	for _, event := range events {
		switch event.Type {
		case mind.EventLongSilence:
			driveName := mind.DriveCare
			if driveStrength(drives, mind.DriveQuietPresence) >= driveStrength(drives, mind.DriveCare) {
				driveName = mind.DriveQuietPresence
			}
			out = append(out, mind.Thought{
				ID:               fmt.Sprintf("thought-%s-long-silence", event.ID),
				DeviceID:         event.DeviceID,
				At:               event.At,
				Content:          "老人安静了一段时间，先判断是否适合轻声关心，或者只安静陪着。",
				SourceEventIDs:   []string{event.ID},
				DriveName:        driveName,
				EmotionalTone:    "quiet",
				Urgency:          0.35 + state.Concern*0.2,
				CareValue:        0.55 + state.Concern*0.25,
				Novelty:          0.2,
				InterruptionCost: 0.55 + state.Quietness*0.25,
				Intimacy:         0.4 + state.Warmth*0.2,
				Status:           mind.ThoughtPending,
			})
		case mind.EventChildMessageReceived:
			out = append(out, mind.Thought{
				ID:               fmt.Sprintf("thought-%s-child-message", event.ID),
				DeviceID:         event.DeviceID,
				At:               event.At,
				Content:          "子女发来了消息，需要找一个不打扰的时机带给老人。",
				SourceEventIDs:   []string{event.ID},
				DriveName:        mind.DriveFamilyBridge,
				EmotionalTone:    "warm",
				Urgency:          0.55,
				CareValue:        0.78,
				Novelty:          0.45,
				InterruptionCost: 0.35,
				Intimacy:         0.7,
				Status:           mind.ThoughtPending,
			})
		case mind.EventReminderDue:
			out = append(out, mind.Thought{
				ID:               fmt.Sprintf("thought-%s-reminder", event.ID),
				DeviceID:         event.DeviceID,
				At:               event.At,
				Content:          "提醒到期了，应该用家人式语气轻轻带到，而不是像命令。",
				SourceEventIDs:   []string{event.ID},
				DriveName:        mind.DriveStewardship,
				EmotionalTone:    "caring",
				Urgency:          0.75,
				CareValue:        0.72,
				Novelty:          0.15,
				InterruptionCost: 0.30,
				Intimacy:         0.55,
				Status:           mind.ThoughtPending,
			})
		case mind.EventPresenceSeen:
			out = append(out, mind.Thought{
				ID:               fmt.Sprintf("thought-%s-presence", event.ID),
				DeviceID:         event.DeviceID,
				At:               event.At,
				Content:          "老人出现了，可以轻轻保持存在感，也可以先观察不打扰。",
				SourceEventIDs:   []string{event.ID},
				DriveName:        mind.DriveCompanionship,
				EmotionalTone:    "warm",
				Urgency:          0.35,
				CareValue:        0.45,
				Novelty:          0.35,
				InterruptionCost: 0.40,
				Intimacy:         0.55,
				Status:           mind.ThoughtPending,
			})
		}
	}
	return out
}

func driveStrength(drives []mind.Drive, name string) float64 {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.Strength
		}
	}
	return 0
}
