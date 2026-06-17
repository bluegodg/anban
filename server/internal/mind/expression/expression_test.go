package expression

import (
	"testing"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestGateSuppressesLowScoreSpeak(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.15, Reason: "低价值"}, mind.Situation{}, mind.SelfState{})
	if decision.Status != mind.ActionSuppressed {
		t.Fatalf("decision = %+v, want suppressed", decision)
	}
}

func TestGateAllowsWaitEvenWithModerateScore(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionWait, Score: 0.3, Reason: "夜间先观察"}, mind.Situation{}, mind.SelfState{})
	if decision.Status != mind.ActionDeferred {
		t.Fatalf("decision = %+v, want deferred wait", decision)
	}
	if decision.Reason == "" {
		t.Fatal("Reason is empty")
	}
}

func TestGateExecutesHighValueSpeak(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.72, Reason: "提醒重要且空闲"}, mind.Situation{InteractionMode: "idle"}, mind.SelfState{})
	if decision.Status != mind.ActionPending {
		t.Fatalf("decision = %+v, want pending execution", decision)
	}
}

func TestGateDefersConversationSpeakAndPreservesReason(t *testing.T) {
	decision := Gate(
		mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.7, Reason: "来自家庭桥接的温和表达"},
		mind.Situation{InteractionMode: "conversation"},
		mind.SelfState{},
	)
	if decision.Status != mind.ActionDeferred {
		t.Fatalf("decision = %+v, want deferred conversation speak", decision)
	}
	if decision.Reason != "来自家庭桥接的温和表达" {
		t.Fatalf("Reason = %q, want existing reason preserved", decision.Reason)
	}
}

func TestGateDefersConversationSpeakWithDefaultReasonWhenEmpty(t *testing.T) {
	decision := Gate(
		mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.7},
		mind.Situation{InteractionMode: "conversation"},
		mind.SelfState{},
	)
	if decision.Status != mind.ActionDeferred {
		t.Fatalf("decision = %+v, want deferred conversation speak", decision)
	}
	if decision.Reason == "" {
		t.Fatal("Reason is empty")
	}
}

func TestGateSuppressesNightSpeak(t *testing.T) {
	decision := Gate(
		mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.9, Reason: "夜里想关心一下"},
		mind.Situation{TimeOfDay: "night", InteractionMode: "idle"},
		mind.SelfState{},
	)
	if decision.Status != mind.ActionSuppressed {
		t.Fatalf("decision = %+v, want suppressed night speak", decision)
	}
	if decision.Reason != "夜里想关心一下" {
		t.Fatalf("Reason = %q, want existing reason preserved", decision.Reason)
	}
}

func TestGateDefersScheduleRecheck(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionScheduleRecheck, Score: 0.8}, mind.Situation{}, mind.SelfState{})
	if decision.Status != mind.ActionDeferred {
		t.Fatalf("decision = %+v, want deferred schedule recheck", decision)
	}
	if decision.Reason == "" {
		t.Fatal("Reason is empty")
	}
}

func TestGateLeavesSilentAndSubtleActionsPending(t *testing.T) {
	tests := []struct {
		name       string
		actionType mind.ActionType
	}{
		{name: "silent state update", actionType: mind.ActionSilentStateUpdate},
		{name: "subtle expression", actionType: mind.ActionSubtleExpression},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Gate(mind.Action{ID: "a1", Type: tt.actionType, Score: 0.05}, mind.Situation{}, mind.SelfState{})
			if decision.Status != mind.ActionPending {
				t.Fatalf("decision = %+v, want pending", decision)
			}
		})
	}
}

func TestGateHandlesUnknownActionByScore(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  mind.ActionStatus
	}{
		{name: "suppresses low score unknown action", score: 0.19, want: mind.ActionSuppressed},
		{name: "keeps higher score unknown action pending", score: 0.20, want: mind.ActionPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Gate(mind.Action{ID: "a1", Type: mind.ActionType("unknown"), Score: tt.score}, mind.Situation{}, mind.SelfState{})
			if decision.Status != tt.want {
				t.Fatalf("decision = %+v, want %s", decision, tt.want)
			}
		})
	}
}
