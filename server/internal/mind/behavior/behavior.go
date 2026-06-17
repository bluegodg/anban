package behavior

import (
	"fmt"
	"math"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Select(s mind.Situation, state mind.SelfState, thoughts []mind.Thought) []mind.Action {
	out := make([]mind.Action, 0, len(thoughts))
	for _, thought := range thoughts {
		actionType, executor := actionFor(thought, s)
		action := mind.Action{
			ID:          fmt.Sprintf("action-%s", thought.ID),
			DeviceID:    thought.DeviceID,
			IntentionID: fmt.Sprintf("intention-%s", thought.ID),
			Type:        actionType,
			Executor:    executor,
			Text:        defaultText(thought, actionType),
			Status:      mind.ActionPending,
			Reason:      reasonFor(thought, actionType),
			Score:       score(thought, s, state, actionType),
		}
		if actionType == mind.ActionScheduleRecheck {
			next := s.At.Add(20 * time.Minute)
			action.ScheduledFor = &next
		}
		out = append(out, action)
	}
	return out
}

func actionFor(thought mind.Thought, s mind.Situation) (mind.ActionType, string) {
	if thought.DriveName == mind.DriveQuietPresence || thought.InterruptionCost >= 0.75 {
		return mind.ActionWait, "mind"
	}

	switch thought.DriveName {
	case mind.DriveStewardship:
		return mind.ActionSpeak, "reminder"
	case mind.DriveFamilyBridge:
		if s.InteractionMode == "conversation" {
			return mind.ActionWait, "mind"
		}
		return mind.ActionSpeak, "message"
	case mind.DriveCompanionship:
		return mind.ActionSpeak, "greeting"
	case mind.DriveCare:
		return mind.ActionScheduleRecheck, "mind"
	default:
		return mind.ActionSilentStateUpdate, "mind"
	}
}

func score(thought mind.Thought, s mind.Situation, state mind.SelfState, actionType mind.ActionType) float64 {
	value := thought.Urgency*0.20 + thought.CareValue*0.25 + thought.Novelty*0.10 + thought.Intimacy*0.15
	personality := state.FamilyWeight*0.08 + state.PetWeight*0.03 + state.StewardWeight*0.05
	cost := thought.InterruptionCost * 0.30

	if s.InteractionMode == "conversation" && actionType == mind.ActionSpeak {
		cost += 0.25
	}
	if s.TimeOfDay == "night" && actionType == mind.ActionSpeak {
		cost += 0.20
	}
	if actionType == mind.ActionWait {
		value += state.Quietness * 0.20
		cost *= 0.5
	}

	return clamp(value + personality - cost)
}

func defaultText(thought mind.Thought, actionType mind.ActionType) string {
	if actionType != mind.ActionSpeak {
		return ""
	}

	switch thought.DriveName {
	case mind.DriveStewardship:
		return "到提醒时间啦，慢慢来，我帮你记着呢。"
	case mind.DriveFamilyBridge:
		return "孩子刚发来一句话，我轻轻说给你听。"
	case mind.DriveCompanionship:
		return "我在这儿呢，慢慢来。"
	default:
		return "我在呢。"
	}
}

func reasonFor(thought mind.Thought, actionType mind.ActionType) string {
	if actionType == mind.ActionWait {
		return "当前打扰成本较高，选择等待或安静陪伴"
	}
	return fmt.Sprintf("由 %s 动机和 thought %s 选择", thought.DriveName, thought.ID)
}

func clamp(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
