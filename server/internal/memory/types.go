package memory

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidInput = errors.New("memory: invalid input")

type Fact struct {
	ID        uint      `gorm:"primaryKey"`
	DeviceID  string    `gorm:"size:64;not null;index;uniqueIndex:idx_memory_fact_device_hash"`
	Hash      string    `gorm:"size:64;not null;uniqueIndex:idx_memory_fact_device_hash"`
	Text      string    `gorm:"size:240;not null"`
	SourceAt  time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Options struct {
	HistoryLimit int
	MaxFacts     int
}

type DistillResult struct {
	DeviceID    string
	AddedFacts  int
	TotalFacts  int
	Degraded    bool
	DegradeNote string
}

type PromptSyncer interface {
	SyncMemoryFacts(ctx context.Context, deviceID string, facts []string) error
}
