package vision

import (
	"encoding/json"
	"errors"
)

const DefaultCaptureTool = "camera.capture"

var ErrInvalidInput = errors.New("vision: invalid input")

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
