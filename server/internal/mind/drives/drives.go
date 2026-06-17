package drives

import (
	"math"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Activate(s mind.Situation, state mind.SelfState, events []mind.Event) []mind.Drive {
	acc := newAccumulator()

	for _, event := range events {
		switch event.Type {
		case mind.EventReminderDue:
			acc.add(mind.DriveStewardship, 0.75+state.StewardWeight, "提醒到期，需要完成照看任务", event.ID)
			acc.add(mind.DriveCare, 0.35+state.Concern*0.35, "提醒和老人状态有关", event.ID)
		case mind.EventChildMessageReceived:
			acc.add(mind.DriveFamilyBridge, 0.80, "子女消息需要温和带到老人身边", event.ID)
			acc.add(mind.DriveCompanionship, 0.25+state.Warmth*0.2, "家庭连接能增强陪伴感", event.ID)
		case mind.EventLongSilence:
			acc.add(mind.DriveCare, 0.35+state.Concern*0.4, "长时间沉默需要留意", event.ID)
			acc.add(mind.DriveQuietPresence, 0.45+state.Quietness*0.4, "当前更适合安静陪伴", event.ID)
		case mind.EventPresenceSeen:
			acc.add(mind.DriveCompanionship, 0.45+state.Warmth*0.25, "看见老人出现，可以维持存在感", event.ID)
			acc.add(mind.DriveCuriosity, 0.30+state.Curiosity*0.25, "新的 presence 值得观察", event.ID)
		}
	}

	for _, loop := range s.OpenLoops {
		if loop == "reminder_due" {
			acc.add(mind.DriveStewardship, 0.25, "当前有未完成提醒", "")
		}
	}
	if s.ActivityLevel == "low" {
		acc.add(mind.DriveQuietPresence, 0.20, "活动水平低时减少打扰", "")
	}

	return acc.drives()
}

type accumulator struct {
	order     []string
	strengths map[string]float64
	reasons   map[string]string
	sourceIDs map[string][]string
	seenIDs   map[string]map[string]bool
}

func newAccumulator() *accumulator {
	return &accumulator{
		strengths: make(map[string]float64),
		reasons:   make(map[string]string),
		sourceIDs: make(map[string][]string),
		seenIDs:   make(map[string]map[string]bool),
	}
}

func (a *accumulator) add(name string, amount float64, reason string, eventID string) {
	if eventID != "" && a.seenIDs[name] != nil && a.seenIDs[name][eventID] {
		return
	}
	if _, ok := a.strengths[name]; !ok {
		a.order = append(a.order, name)
	}
	a.strengths[name] = clamp(a.strengths[name] + amount)
	if a.reasons[name] == "" {
		a.reasons[name] = reason
	}
	if eventID == "" {
		return
	}
	if a.seenIDs[name] == nil {
		a.seenIDs[name] = make(map[string]bool)
	}
	if a.seenIDs[name][eventID] {
		return
	}
	a.sourceIDs[name] = append(a.sourceIDs[name], eventID)
	a.seenIDs[name][eventID] = true
}

func (a *accumulator) drives() []mind.Drive {
	out := make([]mind.Drive, 0, len(a.order))
	for _, name := range a.order {
		out = append(out, mind.Drive{
			Name:           name,
			Strength:       clamp(a.strengths[name]),
			Reason:         a.reasons[name],
			SourceEventIDs: a.sourceIDs[name],
		})
	}
	return out
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
