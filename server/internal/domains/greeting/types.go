package greeting

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("greeting: invalid input")
	ErrNotFound     = errors.New("greeting: not found")
)

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

type ScheduleSlot struct {
	Label      string     `json:"label"`
	Time       string     `json:"time"`
	Enabled    bool       `json:"enabled"`
	TonePreset TonePreset `json:"tonePreset"`
}

type GreetingSchedule struct {
	ID        uint           `gorm:"primaryKey" json:"scheduleId"`
	DeviceID  string         `gorm:"uniqueIndex;not null" json:"deviceId"`
	Slots     []ScheduleSlot `gorm:"serializer:json" json:"slots"`
	CreatedAt time.Time      `json:"-"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type TriggerRequest struct {
	DeviceID   string     `json:"deviceId"`
	TonePreset TonePreset `json:"tonePreset"`
}

type ScheduleRequest struct {
	DeviceID string         `json:"deviceId"`
	Slots    []ScheduleSlot `json:"slots"`
}

type ListFilter struct {
	DeviceID string
	Status   Status
}
