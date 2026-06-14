package status

import (
	"errors"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

var ErrInvalidInput = errors.New("status: invalid input")
var ErrNotFound = errors.New("status: not found")

type GetRequest struct {
	DeviceID string
}

type Snapshot struct {
	DeviceID          string                             `json:"deviceId"`
	Online            bool                               `json:"online"`
	LastSeenAt        *time.Time                         `json:"lastSeenAt,omitempty"`
	LastInteractionAt *time.Time                         `json:"lastInteractionAt,omitempty"`
	Messages          []sharedtypes.MessageStatusSummary `json:"messages"`
}

type SnapshotCache struct {
	ID                uint       `gorm:"primaryKey" json:"-"`
	DeviceID          string     `gorm:"uniqueIndex;not null" json:"deviceId"`
	Online            bool       `json:"online"`
	LastSeenAt        *time.Time `json:"lastSeenAt,omitempty"`
	LastInteractionAt *time.Time `json:"lastInteractionAt,omitempty"`
	CreatedAt         time.Time  `json:"-"`
	UpdatedAt         time.Time  `json:"-"`
}
