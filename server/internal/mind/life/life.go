package life

import (
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

const maxLingeringThoughts = 3

func Update(deviceID string, at time.Time, state mind.SelfState, events []mind.Event) mind.LifeState {
	careFocus := ""
	lingering := []string{}
	seen := map[string]bool{}
	addLingering := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" || seen[text] || len(lingering) >= maxLingeringThoughts {
			return
		}
		seen[text] = true
		lingering = append(lingering, text)
	}
	for _, event := range events {
		switch event.Type {
		case mind.EventLongSilence:
			careFocus = "最近互动偏少，先轻轻留意"
			addLingering(event.Summary)
		case mind.EventChildMessageReceived:
			addLingering("子女消息等待合适时机带到")
		}
	}

	theme := "温和陪伴，少打扰"
	if state.Concern >= 0.7 {
		theme = "轻轻留意老人状态"
	}
	if state.Playfulness >= 0.45 {
		theme = "用更轻松的方式陪着"
	}

	return mind.LifeState{
		DeviceID:                deviceID,
		At:                      at,
		TodayTheme:              theme,
		LingeringThoughts:       lingering,
		SocialEnergy:            clamp(0.45 + state.Energy*0.3 - state.Quietness*0.2),
		CareFocus:               careFocus,
		PlayfulnessTrend:        state.Playfulness,
		RelationshipTemperature: clamp(0.4 + state.Warmth*0.4),
	}
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
