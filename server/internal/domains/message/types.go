package message

import (
	"errors"
	"time"
)

const MaxTextRunes = 100

var ErrInvalidInput = errors.New("message: invalid input")

type Status string

const (
	StatusPending Status = "pending"
	StatusPlayed  Status = "played"
	StatusFailed  Status = "failed"
)

type Message struct {
	ID           uint       `gorm:"primaryKey" json:"messageId"`
	DeviceID     string     `gorm:"index;not null" json:"deviceId"`
	Text         string     `gorm:"size:100;not null" json:"text"`
	FromName     string     `json:"fromName,omitempty"`
	Status       Status     `gorm:"size:20;index;not null" json:"status"`
	QueuedAt     time.Time  `gorm:"index;not null" json:"queuedAt"`
	PlayedAt     *time.Time `json:"playedAt,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `json:"-"`
	UpdatedAt    time.Time  `json:"-"`
}

type SendRequest struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
	FromName string `json:"fromName"`
}

type ListFilter struct {
	DeviceID string
	Status   Status
}
