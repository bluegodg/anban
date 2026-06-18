package message

import (
	"context"
	"errors"
	"time"
)

const MaxTextRunes = 100

var ErrInvalidInput = errors.New("message: invalid input")
var ErrNotFound = errors.New("message: not found")

type Status string

const (
	StatusPending Status = "pending"
	StatusPlayed  Status = "played"
	StatusFailed  Status = "failed"
)

type Message struct {
	ID                uint       `gorm:"primaryKey" json:"messageId"`
	DeviceID          string     `gorm:"index;not null" json:"deviceId"`
	Text              string     `gorm:"size:100;not null" json:"text"`
	FromName          string     `json:"fromName,omitempty"`
	SenderAccountID   *uint      `gorm:"index" json:"senderAccountId,omitempty"`
	SenderDisplayName string     `gorm:"size:80" json:"senderDisplayName,omitempty"`
	SenderRole        string     `gorm:"size:20" json:"senderRole,omitempty"`
	SenderAvatarColor string     `gorm:"size:32" json:"senderAvatarColor,omitempty"`
	Status            Status     `gorm:"size:20;index;not null" json:"status"`
	QueuedAt          time.Time  `gorm:"index;not null" json:"queuedAt"`
	PlayedAt          *time.Time `json:"playedAt,omitempty"`
	ErrorMessage      string     `json:"errorMessage,omitempty"`
	CreatedAt         time.Time  `json:"-"`
	UpdatedAt         time.Time  `json:"-"`
}

type SendRequest struct {
	DeviceID          string `json:"deviceId"`
	Text              string `json:"text"`
	FromName          string `json:"fromName"`
	SenderAccountID   *uint  `json:"-"`
	SenderDisplayName string `json:"-"`
	SenderRole        string `json:"-"`
	SenderAvatarColor string `json:"-"`
}

type ListFilter struct {
	DeviceID string
	Status   Status
}

type MindEvent struct {
	DeviceID string
	Type     string
	SourceID uint
	Summary  string
	Payload  map[string]any
}

type MindSink interface {
	IngestMindEvent(ctx context.Context, event MindEvent) error
}
