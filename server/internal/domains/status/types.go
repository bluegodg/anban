package status

import (
	"errors"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

var ErrInvalidInput = errors.New("status: invalid input")

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
