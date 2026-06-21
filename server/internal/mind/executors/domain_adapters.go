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
