package executors

import (
	"context"
	"errors"

	"github.com/bluegodg/anban/server/internal/mind"
)

var ErrExecutorNotFound = errors.New("mind executors: executor not found")

type Result struct {
	ActionID     string
	Status       mind.ActionStatus
	ExecutorRef  string
	ErrorMessage string
}

type SpeakExecutor interface {
	Speak(ctx context.Context, action mind.Action) (Result, error)
}

type VisionExecutor interface {
	Observe(ctx context.Context, action mind.Action) (Result, error)
}

type PromptExecutor interface {
	SyncPrompt(ctx context.Context, action mind.Action) (Result, error)
}

type Dispatcher struct {
	speakers map[string]SpeakExecutor
	visions  map[string]VisionExecutor
}

func NewDispatcher(speakers map[string]SpeakExecutor) *Dispatcher {
	copied := make(map[string]SpeakExecutor, len(speakers))
	for name, speaker := range speakers {
		copied[name] = speaker
	}
	return &Dispatcher{speakers: copied, visions: make(map[string]VisionExecutor)}
}

func (d *Dispatcher) RegisterVision(name string, executor VisionExecutor) {
	if d == nil || executor == nil {
		return
	}
	d.visions[name] = executor
}

func (d *Dispatcher) Execute(ctx context.Context, action mind.Action) (Result, error) {
	switch action.Type {
	case mind.ActionWait, mind.ActionScheduleRecheck:
		return Result{ActionID: action.ID, Status: mind.ActionDeferred, ExecutorRef: "mind"}, nil
	case mind.ActionSilentStateUpdate:
		return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "mind"}, nil
	case mind.ActionSpeak:
		exec, ok := d.speakers[action.Executor]
		if !ok || exec == nil {
			return missingExecutorResult(action), ErrExecutorNotFound
		}
		return exec.Speak(ctx, action)
	case mind.ActionCallMCPTool:
		exec, ok := d.visions[action.Executor]
		if !ok || exec == nil {
			return missingExecutorResult(action), ErrExecutorNotFound
		}
		return exec.Observe(ctx, action)
	default:
		return missingExecutorResult(action), ErrExecutorNotFound
	}
}

func missingExecutorResult(action mind.Action) Result {
	return Result{
		ActionID:     action.ID,
		Status:       mind.ActionFailed,
		ErrorMessage: ErrExecutorNotFound.Error(),
	}
}
