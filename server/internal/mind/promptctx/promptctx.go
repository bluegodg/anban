package promptctx

import (
	"strings"

	"github.com/bluegodg/anban/server/internal/mind"
)

const maxContextRunes = 360

type CompanionContext struct {
	DisplayName      string
	ProfileSummaries []string
	MemoryFacts      []string
}

func Build(state mind.SelfState, recent []mind.Event) string {
	return BuildWithCompanion(state, recent, CompanionContext{})
}

func BuildWithCompanion(state mind.SelfState, recent []mind.Event, companion CompanionContext) string {
	lines := make([]string, 0, 7)
	if state.Concern >= 0.70 {
		lines = append(lines, "最近你较挂念老人(concern 偏高)，语气更关切些")
	}
	if state.Warmth >= 0.65 {
		lines = append(lines, "关系温度较暖，回答可以更亲近自然")
	}
	if state.Quietness >= 0.70 {
		lines = append(lines, "老人近期偏安静，少追问，先轻声陪着")
	}
	if topics := recentTopics(recent); len(topics) > 0 {
		lines = append(lines, "今天聊过/留意过："+strings.Join(topics, "；"))
	}
	if displayName := strings.TrimSpace(companion.DisplayName); displayName != "" {
		lines = append(lines, "陪伴对象："+displayName)
	}
	if summaries := firstNonEmpty(companion.ProfileSummaries, 2); len(summaries) > 0 {
		lines = append(lines, "画像重点："+strings.Join(summaries, "；"))
	}
	if facts := firstNonEmpty(companion.MemoryFacts, 2); len(facts) > 0 {
		lines = append(lines, "记忆重点："+strings.Join(facts, "；"))
	}
	if len(lines) == 0 {
		return ""
	}
	return truncateRunes(strings.Join(lines, "；"), maxContextRunes)
}

func firstNonEmpty(values []string, limit int) []string {
	out := make([]string, 0, limit)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func recentTopics(recent []mind.Event) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, event := range recent {
		text := topicText(event)
		if text == "" || seen[text] {
			continue
		}
		out = append(out, text)
		seen[text] = true
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func topicText(event mind.Event) string {
	switch event.Type {
	case mind.EventElderSpoke:
		if text := payloadString(event.Payload, "text"); text != "" {
			return truncateRunes(text, 22)
		}
		return truncateRunes(trimKnownPrefix(event.Summary), 22)
	case mind.EventLongSilence, mind.EventReminderDue, mind.EventReminderAcknowledged, mind.EventChildMessageReceived:
		return truncateRunes(trimKnownPrefix(event.Summary), 22)
	default:
		return ""
	}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func trimKnownPrefix(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"老人说：", "安伴回应："} {
		value = strings.TrimPrefix(value, prefix)
	}
	return strings.TrimSpace(value)
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
