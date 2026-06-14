package vision

import (
	"encoding/json"
	"errors"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

// DefaultCaptureTool 是真机 ESP32 上报的拍照 MCP 工具名（2026-06-04 真机日志确认为 self.camera.take_photo）。
const DefaultCaptureTool = "self.camera.take_photo"

var ErrInvalidInput = errors.New("vision: invalid input")
var ErrPresenceUnavailable = errors.New("vision: presence unavailable")

type CaptureRequest struct {
	DeviceID string         `json:"deviceId"`
	Tool     string         `json:"tool,omitempty"`
	Args     map[string]any `json:"args,omitempty"`
}

type CaptureResult struct {
	DeviceID string          `json:"deviceId"`
	Tool     string          `json:"tool"`
	Raw      json.RawMessage `json:"raw"`
}

type Presence string

const (
	PresenceUnknown Presence = "unknown"
	PresenceSomeone Presence = "someone"
	PresenceNoOne   Presence = "no_one"
)

type PresenceObservationRequest struct {
	DeviceID   string    `json:"deviceId"`
	Presence   Presence  `json:"presence"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
}

type PresenceObservationResult struct {
	DeviceID          string                               `json:"deviceId"`
	PreviousPresence  Presence                             `json:"previousPresence"`
	Presence          Presence                             `json:"presence"`
	ObservedAt        time.Time                            `json:"observedAt"`
	TriggeredGreeting bool                                 `json:"triggeredGreeting"`
	Greeting          *sharedtypes.ProactiveGreetingResult `json:"greeting,omitempty"`
}

type PresenceCheckResult struct {
	Capture     CaptureResult             `json:"capture"`
	Observation PresenceObservationResult `json:"observation"`
}
