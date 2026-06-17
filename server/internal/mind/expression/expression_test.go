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
