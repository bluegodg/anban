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

type HistoryRequest struct {
	DeviceID string
	Limit    int
}

type Snapshot struct {
	DeviceID          string                             `json:"deviceId"`
	Online            bool                               `json:"online"`
	LastSeenAt        *time.Time                         `json:"lastSeenAt,omitempty"`
	LastInteractionAt *time.Time                         `json:"lastInteractionAt,omitempty"`
	Messages          []sharedtypes.MessageStatusSummary `json:"messages"`
}

type HistoryResponse struct {
	DeviceID string         `json:"deviceId"`
	Messages []HistoryEntry `json:"messages"`
}

type HistoryEntry struct {
	Role string     `json:"role"`
	Text string     `json:"text"`
	At   *time.Time `json:"at,omitempty"`
}

type DevicePanel struct {
	DeviceID     string         `json:"deviceId"`
	DeviceCode   string         `json:"deviceCode,omitempty"`
	Online       bool           `json:"online"`
	LastActiveAt *time.Time     `json:"lastActiveAt,omitempty"`
	Volume       *int           `json:"volume,omitempty"`
	Brightness   *int           `json:"brightness,omitempty"`
	Theme        string         `json:"theme,omitempty"`
	Battery      *int           `json:"battery,omitempty"`
	Network      *DeviceNetwork `json:"network,omitempty"`
}

type DeviceNetwork struct {
	Type   string `json:"type,omitempty"`
	SSID   string `json:"ssid,omitempty"`
	Signal string `json:"signal,omitempty"`
}

type SetVolumeRequest struct {
	DeviceID string
	Volume   int
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
