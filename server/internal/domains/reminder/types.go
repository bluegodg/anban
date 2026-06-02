package reminder

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("reminder: invalid input")
	ErrNotFound     = errors.New("reminder: not found")
)

type Category string

const (
	CategoryMed      Category = "med"
	CategoryBirthday Category = "birthday"
	CategoryFestival Category = "festival"
	CategoryCustom   Category = "custom"
)

type Status string

const (
	StatusScheduled  Status = "scheduled"
	StatusPlayed     Status = "played"
	StatusCompleted  Status = "completed"
	StatusUnanswered Status = "unanswered"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

type AckKind string

const (
	AckKindVoice   AckKind = "voice"
	AckKindTimeout AckKind = "timeout"
)

type Reminder struct {
	ID             uint       `gorm:"primaryKey" json:"reminderId"`
	DeviceID       string     `gorm:"index;not null" json:"deviceId"`
	ScheduledAt    time.Time  `gorm:"index;not null" json:"scheduledAt"`
	Content        string     `gorm:"size:120;not null" json:"content"`
	Category       Category   `gorm:"size:20;not null" json:"category"`
	Text           string     `gorm:"size:160;not null" json:"text"`
	Status         Status     `gorm:"size:20;index;not null" json:"status"`
	JobID          string     `gorm:"size:64" json:"jobId,omitempty"`
	AckJobID       string     `gorm:"size:64" json:"ackJobId,omitempty"`
	PlayedAt       *time.Time `json:"playedAt,omitempty"`
	AckKind        AckKind    `gorm:"size:20" json:"ackKind,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	CreatedAt      time.Time  `json:"-"`
	UpdatedAt      time.Time  `json:"-"`
}

type CreateRequest struct {
	DeviceID    string    `json:"deviceId"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Content     string    `json:"content"`
	Category    Category  `json:"category"`
}

type ListFilter struct {
	DeviceID string
	Status   Status
}

type AckRequest struct {
	AckKind AckKind `json:"ackKind"`
}
