package executors

import (
	"context"

	"github.com/bluegodg/anban/server/internal/mind"
)

type SpeakFunc func(ctx context.Context, action mind.Action) (Result, error)

func (fn SpeakFunc) Speak(ctx context.Context, action mind.Action) (Result, error) {
	return fn(ctx, action)
}

type VisionFunc func(ctx context.Context, action mind.Action) (Result, error)

func (fn VisionFunc) Observe(ctx context.Context, action mind.Action) (Result, error) {
	return fn(ctx, action)
}

// DelegatedSpeak returns a SpeakExecutor for speak intents that the mind does
// not voice itself, because a dedicated domain channel already delivers them
// (reminders via the reminder service, child messages via the message service).
// It records the choice as a clean deferral, so the child-facing mind view never
// shows a raw "executor not found" error for these always-handled-elsewhere intents.
func DelegatedSpeak(channel string) SpeakExecutor {
	return SpeakFunc(func(_ context.Context, action mind.Action) (Result, error) {
		return Result{ActionID: action.ID, Status: mind.ActionDeferred, ExecutorRef: channel}, nil
	})
}
