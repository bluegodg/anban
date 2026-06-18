package memory

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("memory: invalid input")
	ErrNotFound     = errors.New("memory: not found")
)

type Fact struct {
	ID        uint      `json:"factId" gorm:"primaryKey"`
	DeviceID  string    `json:"deviceId" gorm:"size:64;not null;index;uniqueIndex:idx_memory_fact_device_hash"`
	Hash      string    `json:"-" gorm:"size:64;not null;uniqueIndex:idx_memory_fact_device_hash"`
	Text      string    `json:"text" gorm:"size:240;not null"`
	Source    string    `json:"source" gorm:"size:32;not null;default:dialogue"`
	SourceAt  time.Time `json:"sourceAt" gorm:"index"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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

type FactRequest struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}
