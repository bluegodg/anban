package behavior

import (
	"crypto/sha1"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

const proactiveConcernThreshold = 0.50
const autonomousVisionConcernThreshold = 0.70

func Select(s mind.Situation, state mind.SelfState, thoughts []mind.Thought) []mind.Action {
	out := make([]mind.Action, 0, len(thoughts))
	thoughtIDCounts := countThoughtIDs(thoughts)
	reservedActionIDs, reservedIntentionIDs := reservedIDs(thoughtIDCounts)
	usedActionIDs := map[string]bool{}
	usedIntentionIDs := map[string]bool{}

	for i, thought := range thoughts {
		actionType, executor := actionFor(thought, s, state)
		identity := actionIdentity(s, thought, i, thoughtIDCounts, usedActionIDs, usedIntentionIDs, reservedActionIDs, reservedIntentionIDs)
		action := mind.Action{
			ID:          fmt.Sprintf("action-%s", identity),
			DeviceID:    deviceIDFor(thought, s),
			IntentionID: fmt.Sprintf("intention-%s", identity),
			Type:        actionType,
			Executor:    executor,
			Text:        defaultText(thought, actionType),
			Args:        argsFor(thought, actionType, executor),
			Status:      mind.ActionPending,
			Reason:      reasonFor(thought, actionType, s, state),
			Score:       score(thought, s, state, actionType),
		}
		if actionType == mind.ActionScheduleRecheck {
			if base, ok := scheduleBase(thought, s); ok {
				next := base.Add(20 * time.Minute)
				action.ScheduledFor = &next
			}
		}
		usedActionIDs[action.ID] = true
		usedIntentionIDs[action.IntentionID] = true
		out = append(out, action)
	}
	return out
}

func countThoughtIDs(thoughts []mind.Thought) map[string]int {
	counts := map[string]int{}
	for _, thought := range thoughts {
		if thought.ID == "" {
			continue
		}
		counts[thought.ID]++
	}
	return counts
}

func reservedIDs(thoughtIDCounts map[string]int) (map[string]bool, map[string]bool) {
	actionIDs := map[string]bool{}
	intentionIDs := map[string]bool{}
	for id, count := range thoughtIDCounts {
		if count != 1 {
			continue
		}
		actionIDs[fmt.Sprintf("action-%s", id)] = true
		intentionIDs[fmt.Sprintf("intention-%s", id)] = true
	}
	return actionIDs, intentionIDs
}

func actionIdentity(
	s mind.Situation,
	thought mind.Thought,
	index int,
	thoughtIDCounts map[string]int,
	usedActionIDs map[string]bool,
	usedIntentionIDs map[string]bool,
	reservedActionIDs map[string]bool,
	reservedIntentionIDs map[string]bool,
) string {
	if thought.ID != "" && thoughtIDCounts[thought.ID] == 1 {
		return thought.ID
	}

	base := fallbackIdentity(s, thought, index)
	for attempt := 0; ; attempt++ {
		candidate := base
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d", base, attempt)
		}
		actionID := fmt.Sprintf("action-%s", candidate)
		intentionID := fmt.Sprintf("intention-%s", candidate)
		if usedActionIDs[actionID] || usedIntentionIDs[intentionID] {
			continue
		}
		if reservedActionIDs[actionID] || reservedIntentionIDs[intentionID] {
			continue
		}
		return candidate
	}
}

func fallbackIdentity(s mind.Situation, thought mind.Thought, index int) string {
	at := thought.At
	if at.IsZero() {
		at = s.At
	}
	seed := fmt.Sprintf("%s|%s|%d|%d", deviceIDFor(thought, s), thought.DriveName, at.UnixNano(), index)
	sum := sha1.Sum([]byte(seed))
	return fmt.Sprintf("fallback-%x-%d", sum[:6], index)
}

func deviceIDFor(thought mind.Thought, s mind.Situation) string {
	if thought.DeviceID != "" {
		return thought.DeviceID
	}
	return s.DeviceID
}

func scheduleBase(thought mind.Thought, s mind.Situation) (time.Time, bool) {
	if !s.At.IsZero() {
		return s.At, true
	}
	if !thought.At.IsZero() {
		return thought.At, true
	}
	return time.Time{}, false
}

func actionFor(thought mind.Thought, s mind.Situation, state mind.SelfState) (mind.ActionType, string) {
	if thought.DriveName == mind.DriveQuietPresence {
		if shouldSpeakQuietPresence(thought, s, state) {
			return mind.ActionSpeak, "greeting"
		}
		return mind.ActionWait, "mind"
	}
	if (thought.DriveName == mind.DriveCare || thought.DriveName == mind.DriveCuriosity) &&
		thought.InterruptionCost <= 0.85 && shouldObserve(thought, s, state) {
		return mind.ActionCallMCPTool, "vision"
	}
	if thought.InterruptionCost >= 0.75 {
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
	case mind.DriveCare, mind.DriveCuriosity:
		return mind.ActionScheduleRecheck, "mind"
	default:
		return mind.ActionSilentStateUpdate, "mind"
	}
}

func shouldObserve(thought mind.Thought, s mind.Situation, state mind.SelfState) bool {
	if hasConstraint(s, mind.ConstraintMindAutonomousVisionDisabled) ||
		hasConstraint(s, mind.ConstraintMindProactiveDaytimeOnly) ||
		hasConstraint(s, mind.ConstraintMindAutonomousVisionCooldownActive) {
		return false
	}
	return state.Concern >= autonomousVisionConcernThreshold ||
		thought.CareValue >= 0.75 ||
		state.Curiosity >= 0.75
}

func shouldSpeakQuietPresence(thought mind.Thought, s mind.Situation, state mind.SelfState) bool {
	if thought.InterruptionCost >= 0.85 {
		return false
	}
	if hasConstraint(s, mind.ConstraintMindProactiveDaytimeOnly) {
		return false
	}
	if hasConstraint(s, mind.ConstraintMindProactiveCooldownActive) {
		return false
	}
	if state.Concern < proactiveConcernThreshold && thought.CareValue < 0.55 {
		return false
	}
	return true
}

func hasConstraint(s mind.Situation, constraint string) bool {
	for _, value := range s.Constraints {
		if value == constraint {
			return true
		}
	}
	return false
}

func score(thought mind.Thought, s mind.Situation, state mind.SelfState, actionType mind.ActionType) float64 {
	value := thought.Urgency*0.20 + thought.CareValue*0.25 + thought.Novelty*0.10 + thought.Intimacy*0.15
	personality := state.FamilyWeight*0.08 + state.PetWeight*0.03 + state.StewardWeight*0.05
	cost := thought.InterruptionCost * 0.30
	if thought.DriveName == mind.DriveQuietPresence && actionType == mind.ActionSpeak {
		value += state.Concern * 0.18
		cost *= 0.65
	}
	if actionType == mind.ActionCallMCPTool {
		value += state.Concern * 0.18
		cost *= 0.75
	}

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
	case mind.DriveQuietPresence:
		return "我在这儿呢，刚想起你，今天还顺心吗？"
	case mind.DriveCompanionship:
		return "我在这儿呢，慢慢来。"
	default:
		return "我在呢。"
	}
}

func argsFor(thought mind.Thought, actionType mind.ActionType, executor string) map[string]any {
	if thought.DriveName == mind.DriveQuietPresence && actionType == mind.ActionSpeak && executor == "greeting" {
		return map[string]any{"mindProactive": true}
	}
	if actionType == mind.ActionCallMCPTool && executor == "vision" {
		return map[string]any{
			"mindAutonomousVision": true,
			"tool":                 "self.camera.take_photo",
		}
	}
	return nil
}

func reasonFor(thought mind.Thought, actionType mind.ActionType, s mind.Situation, state mind.SelfState) string {
	if (thought.DriveName == mind.DriveCare || thought.DriveName == mind.DriveCuriosity) && actionType == mind.ActionScheduleRecheck {
		switch {
		case hasConstraint(s, mind.ConstraintMindAutonomousVisionDisabled):
			return "自主视觉已关闭，稍后再确认状态"
		case hasConstraint(s, mind.ConstraintMindProactiveDaytimeOnly):
			return "自主观察仅在白天启用，稍后再确认状态"
		case hasConstraint(s, mind.ConstraintMindAutonomousVisionCooldownActive):
			return "仍在自主视觉冷却期内，稍后再确认状态"
		default:
			return "关心或好奇强度不足，稍后再确认状态"
		}
	}
	if actionType == mind.ActionCallMCPTool && thought.DriveName == mind.DriveCare {
		return "长时间安静且关心强度较高，先通过摄像头确认老人状态"
	}
	if actionType == mind.ActionCallMCPTool && thought.DriveName == mind.DriveCuriosity {
		return "出现值得确认的新情况，先通过摄像头安静观察"
	}
	if actionType == mind.ActionWait {
		if thought.DriveName == mind.DriveCare || thought.DriveName == mind.DriveCuriosity {
			switch {
			case hasConstraint(s, mind.ConstraintMindAutonomousVisionDisabled):
				return "自主视觉已关闭，选择等待"
			case hasConstraint(s, mind.ConstraintMindProactiveDaytimeOnly):
				return "自主观察仅在白天启用，夜间选择等待"
			case hasConstraint(s, mind.ConstraintMindAutonomousVisionCooldownActive):
				return "仍在自主视觉冷却期内，选择等待"
			}
		}
		if thought.DriveName == mind.DriveQuietPresence && hasConstraint(s, mind.ConstraintMindProactiveDaytimeOnly) {
			return "自主开口仅白天启用，夜间选择等待"
		}
		if thought.DriveName == mind.DriveQuietPresence && hasConstraint(s, mind.ConstraintMindProactiveCooldownActive) {
			return "仍在自主开口冷却期内，选择等待"
		}
		if thought.DriveName == mind.DriveQuietPresence && state.Concern < proactiveConcernThreshold {
			return "关心强度不高，选择安静陪伴"
		}
		return "当前打扰成本较高，选择等待或安静陪伴"
	}
	if thought.DriveName == mind.DriveQuietPresence && actionType == mind.ActionSpeak {
		return "长时间沉默且关心强度较高，轻声确认老人状态"
	}
	if actionType == mind.ActionSpeak && thought.DriveName == mind.DriveStewardship {
		return "到点的关怀提醒，交给提醒按时送达。"
	}
	if actionType == mind.ActionSpeak && thought.DriveName == mind.DriveFamilyBridge {
		return "把家人的话带到，交给留言通道送达。"
	}
	return fmt.Sprintf("由 %s 动机和 thought %s 选择", strings.TrimSpace(thought.DriveName), thought.ID)
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
