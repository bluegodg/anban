package greeting

import (
	"errors"
	"time"
)

var ErrInvalidInput = errors.New("greeting: invalid input")

type TonePreset string

const (
	ToneWarm   TonePreset = "warm"
	ToneCasual TonePreset = "casual"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusPlayed  Status = "played"
	StatusFailed  Status = "failed"
)

type Greeting struct {
	ID           uint       `gorm:"primaryKey" json:"greetingId"`
	DeviceID     string     `gorm:"index;not null" json:"deviceId"`
	TonePreset   TonePreset `gorm:"size:20;not null" json:"tonePreset"`
	Text         string     `gorm:"size:120;not null" json:"text"`
	Status       Status     `gorm:"size:20;index;not null" json:"status"`
	TriggeredAt  time.Time  `gorm:"index;not null" json:"triggeredAt"`
	PlayedAt     *time.Time `json:"playedAt,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `json:"-"`
	UpdatedAt    time.Time  `json:"-"`
}

type TriggerRequest struct {
	DeviceID   string     `json:"deviceId"`
	TonePreset TonePreset `json:"tonePreset"`
}

type ListFilter struct {
	DeviceID string
	Status   Status
}
