package llm

import (
	"context"
	"time"
)

type Message struct {
	Role string
	Text string
	At   time.Time
}

type FactExtractionRequest struct {
	DeviceID      string
	Messages      []Message
	ExistingFacts []string
	Limit         int
}

type FactExtractor interface {
	ExtractFacts(ctx context.Context, req FactExtractionRequest) ([]string, error)
}

type PortraitRequest struct {
	ProfileContext   string
	MemoryFacts      []string
	PreviousPortrait string
}
