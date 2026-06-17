package expression

import "github.com/bluegodg/anban/server/internal/mind"

func Gate(action mind.Action, s mind.Situation, state mind.SelfState) mind.Action {
	switch action.Type {
	case mind.ActionWait, mind.ActionScheduleRecheck:
		action.Status = mind.ActionDeferred
		if action.Reason == "" {
			action.Reason = "选择等待，避免打扰"
		}
		return action
	case mind.ActionSilentStateUpdate, mind.ActionSubtleExpression:
		action.Status = mind.ActionPending
		return action
	case mind.ActionSpeak:
		if s.InteractionMode == "conversation" && action.Score < 0.85 {
			action.Status = mind.ActionDeferred
			action.Reason = "老人正在对话，主动表达延后"
			return action
		}
		if action.Score < 0.35 {
			action.Status = mind.ActionSuppressed
			if action.Reason == "" {
				action.Reason = "表达价值不足"
			}
			return action
		}
		action.Status = mind.ActionPending
		return action
	default:
		if action.Score < 0.20 {
			action.Status = mind.ActionSuppressed
			return action
		}
		action.Status = mind.ActionPending
		return action
	}
}
