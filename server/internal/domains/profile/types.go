package profile

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("profile: invalid input")
	ErrNotFound     = errors.New("profile: not found")
)

const (
	PortraitModeAuto   = "auto"
	PortraitModeManual = "manual"
)

type Fields struct {
	Name           string   `json:"name" gorm:"size:100"`
	Nickname       string   `json:"nickname" gorm:"size:100"`
	Children       []string `json:"children" gorm:"serializer:json"`
	Grandchildren  []string `json:"grandchildren" gorm:"serializer:json"`
	Hobbies        []string `json:"hobbies" gorm:"serializer:json"`
	Schedule       string   `json:"schedule" gorm:"type:text"`
	Health         string   `json:"health" gorm:"type:text"`
	Taboos         []string `json:"taboos" gorm:"serializer:json"`
	AIPortrait     string   `json:"aiPortrait" gorm:"type:text"`
	AIPortraitMode string   `json:"aiPortraitMode" gorm:"size:16;default:auto"`
}

type Profile struct {
	ID                  uint       `json:"profileId" gorm:"primaryKey"`
	DeviceID            string     `json:"deviceId" gorm:"uniqueIndex;not null"`
	Fields              Fields     `json:"fields" gorm:"embedded"`
	MemoryFacts         []string   `json:"memoryFacts" gorm:"serializer:json"`
	MindContext         string     `json:"mindContext" gorm:"type:text"`
	Prompt              string     `json:"prompt" gorm:"type:text"`
	AIPortraitInputHash string     `json:"-" gorm:"size:64"`
	AIPortraitUpdatedAt *time.Time `json:"aiPortraitUpdatedAt,omitempty"`
	CreatedAt           time.Time  `json:"-"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type PortraitInput struct {
	Fields           Fields
	MemoryFacts      []string
	PreviousPortrait string
}

type PortraitGenerator interface {
	GeneratePortrait(ctx context.Context, input PortraitInput) (string, error)
}

type UpdateRequest struct {
	DeviceID string `json:"deviceId"`
	Fields   Fields `json:"fields"`
}
