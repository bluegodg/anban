package promptctx

import (
	"strings"

	"github.com/bluegodg/anban/server/internal/mind"
)

const maxContextRunes = 220

func Build(state mind.SelfState, recent []mind.Event) string {
	lines := make([]string, 0, 4)
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
	if len(lines) == 0 {
		return ""
	}
	return truncateRunes(strings.Join(lines, "；"), maxContextRunes)
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
