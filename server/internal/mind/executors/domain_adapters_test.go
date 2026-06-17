package executors

import (
	"context"
	"errors"
	"testing"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestDispatcherRoutesSpeakActionToNamedExecutor(t *testing.T) {
	rem := &fakeSpeakExecutor{}
	msg := &fakeSpeakExecutor{}
	dispatcher := NewDispatcher(map[string]SpeakExecutor{"reminder": rem, "message": msg})

	result, err := dispatcher.Execute(context.Background(), mind.Action{
		ID:       "action-1",
		Type:     mind.ActionSpeak,
		Executor: "reminder",
		DeviceID: "dev-001",
		Text:     "到提醒时间啦",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != mind.ActionExecuted {
		t.Fatalf("result = %+v, want executed", result)
	}
	if rem.calls != 1 || msg.calls != 0 {
		t.Fatalf("rem calls=%d msg calls=%d, want reminder only", rem.calls, msg.calls)
	}
}

func TestDispatcherDefersWaitActionWithoutDomainCall(t *testing.T) {
	dispatcher := NewDispatcher(map[string]SpeakExecutor{"reminder": &fakeSpeakExecutor{}})
	result, err := dispatcher.Execute(context.Background(), mind.Action{
		ID:       "action-1",
		Type:     mind.ActionWait,
		Executor: "mind",
		DeviceID: "dev-001",
	})
	if err != nil {
		t.Fatalf("Execute wait: %v", err)
	}
	if result.Status != mind.ActionDeferred {
		t.Fatalf("result = %+v, want deferred", result)
	}
}

func TestDispatcherFailsWhenSpeakExecutorMissing(t *testing.T) {
	dispatcher := NewDispatcher(nil)
	result, err := dispatcher.Execute(context.Background(), mind.Action{
		ID:       "action-1",
		Type:     mind.ActionSpeak,
		Executor: "message",
		DeviceID: "dev-001",
	})
	if !errors.Is(err, ErrExecutorNotFound) {
		t.Fatalf("Execute error = %v, want ErrExecutorNotFound", err)
	}
	if result.Status != mind.ActionFailed || result.ErrorMessage == "" {
		t.Fatalf("result = %+v, want failed with error message", result)
	}
}

func TestSpeakFuncAdaptsFunction(t *testing.T) {
	called := false
	adapter := SpeakFunc(func(ctx context.Context, action mind.Action) (Result, error) {
		called = true
		return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "fn-ref"}, nil
	})

	result, err := adapter.Speak(context.Background(), mind.Action{ID: "action-1"})
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if !called || result.ExecutorRef != "fn-ref" {
		t.Fatalf("called=%v result=%+v, want function result", called, result)
	}
}

type fakeSpeakExecutor struct{ calls int }

func (f *fakeSpeakExecutor) Speak(ctx context.Context, action mind.Action) (Result, error) {
	f.calls++
	return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "fake-ref"}, nil
}
