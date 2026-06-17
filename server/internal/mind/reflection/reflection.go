package reflection

import (
	"fmt"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Summarize(deviceID string, at time.Time, feedback []mind.Feedback) mind.Reflection {
	adjustments := map[string]float64{}
	lessons := []string{}
	summary := "本轮互动没有明显反馈"
	for _, item := range feedback {
		switch item.Signal {
		case "user_replied", "conversation_continued":
			adjustments["warmth"] += 0.03
			adjustments["quietness"] -= 0.01
			lessons = append(lessons, "当前表达被接受，类似时机可保持轻声互动")
			summary = "老人接受了本轮互动"
		case "user_ignored":
			adjustments["quietness"] += 0.03
			lessons = append(lessons, "类似场景减少追问，优先等待")
			summary = "老人没有回应本轮互动"
		case "user_rejected":
			adjustments["quietness"] += 0.06
			adjustments["playfulness"] -= 0.03
			lessons = append(lessons, "类似表达需要更克制")
			summary = "老人拒绝了本轮互动"
		}
	}
	return mind.Reflection{
		ID:               fmt.Sprintf("reflection-%s-%d", deviceID, at.UnixNano()),
		DeviceID:         deviceID,
		At:               at,
		EpisodeSummary:   summary,
		StateAdjustments: adjustments,
		BehaviorLessons:  lessons,
	}
}
