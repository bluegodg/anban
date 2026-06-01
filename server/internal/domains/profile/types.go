package profile

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput = errors.New("profile: invalid input")
	ErrNotFound     = errors.New("profile: not found")
)

type Fields struct {
	Name          string   `json:"name" gorm:"size:100"`
	Nickname      string   `json:"nickname" gorm:"size:100"`
	Children      []string `json:"children" gorm:"serializer:json"`
	Grandchildren []string `json:"grandchildren" gorm:"serializer:json"`
	Hobbies       []string `json:"hobbies" gorm:"serializer:json"`
	Schedule      string   `json:"schedule" gorm:"type:text"`
	Health        string   `json:"health" gorm:"type:text"`
	Taboos        []string `json:"taboos" gorm:"serializer:json"`
}

type Profile struct {
	ID        uint      `json:"profileId" gorm:"primaryKey"`
	DeviceID  string    `json:"deviceId" gorm:"uniqueIndex;not null"`
	Fields    Fields    `json:"fields" gorm:"embedded"`
	Prompt    string    `json:"prompt" gorm:"type:text"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UpdateRequest struct {
	DeviceID string `json:"deviceId"`
	Fields   Fields `json:"fields"`
}
