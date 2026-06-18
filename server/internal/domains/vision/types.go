package vision

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

// DefaultCaptureTool 是真机 ESP32 上报的拍照 MCP 工具名（2026-06-04 真机日志确认为 self.camera.take_photo）。
const DefaultCaptureTool = "self.camera.take_photo"

var ErrInvalidInput = errors.New("vision: invalid input")
var ErrPresenceUnavailable = errors.New("vision: presence unavailable")
var ErrNotFound = errors.New("vision: capture not found")
var ErrCaptureExpired = errors.New("vision: capture expired")
var ErrCaptureInProgress = errors.New("vision: capture in progress")
var ErrStoreUnavailable = errors.New("vision: store unavailable")
var ErrImageUploadInvalid = errors.New("vision: image upload invalid")
var ErrImageTooLarge = errors.New("vision: image too large")

type CaptureStatus string

const (
	CaptureStatusPending   CaptureStatus = "pending"
	CaptureStatusSucceeded CaptureStatus = "succeeded"
	CaptureStatusPartial   CaptureStatus = "partial"
	CaptureStatusFailed    CaptureStatus = "failed"
	CaptureStatusExpired   CaptureStatus = "expired"
)

type Capture struct {
	ID                uint          `gorm:"primaryKey" json:"-"`
	CaptureID         string        `gorm:"uniqueIndex;size:80;not null" json:"captureId"`
	DeviceID          string        `gorm:"index;not null" json:"deviceId"`
	Status            CaptureStatus `gorm:"size:20;index;not null" json:"status"`
	ImageRelativePath string        `json:"-"`
	ImageContentType  string        `gorm:"size:80" json:"imageContentType,omitempty"`
	ImageSize         int64         `json:"imageSize,omitempty"`
	ImageSHA256       string        `gorm:"size:64" json:"imageSha256,omitempty"`
	AnalysisSummary   string        `json:"analysisSummary,omitempty"`
	AnalysisRaw       string        `json:"analysisRaw,omitempty"`
	Presence          Presence      `gorm:"size:20;not null" json:"presence"`
	ConcernsJSON      string        `gorm:"type:text" json:"-"`
	FailureCode       string        `gorm:"size:80" json:"failureCode,omitempty"`
	FailureMessage    string        `json:"failureMessage,omitempty"`
	CapturedAt        *time.Time    `json:"capturedAt,omitempty"`
	ExpiresAt         time.Time     `gorm:"index;not null" json:"expiresAt"`
	CreatedAt         time.Time     `json:"-"`
	UpdatedAt         time.Time     `json:"-"`
}

func (Capture) TableName() string {
	return "vision_captures"
}

type LookRequest struct {
	DeviceID string `json:"deviceId"`
}

type CaptureListRequest struct {
	DeviceID string
	Limit    int
}

type ReanalyzeRequest struct {
	DeviceID  string
	CaptureID string
}

type CaptureAnalysis struct {
	Summary  string   `json:"summary"`
	Presence Presence `json:"presence"`
	Concerns []string `json:"concerns"`
}

type CaptureDTO struct {
	CaptureID      string          `json:"captureId"`
	DeviceID       string          `json:"deviceId,omitempty"`
	Status         CaptureStatus   `json:"status"`
	CapturedAt     *time.Time      `json:"capturedAt,omitempty"`
	ImageURL       string          `json:"imageUrl,omitempty"`
	Analysis       CaptureAnalysis `json:"analysis"`
	FailureCode    string          `json:"failureCode,omitempty"`
	FailureMessage string          `json:"failureMessage,omitempty"`
}

type DeviceVisionUpload struct {
	DeviceID      string
	ClientID      string
	Authorization string
	Question      string
	FileName      string
	ContentType   string
	Image         []byte
}

type CaptureImage struct {
	Bytes       []byte
	ContentType string
	Size        int64
	SHA256      string
}

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

type PresencePollResult struct {
	DeviceID   string              `json:"deviceId"`
	Skipped    bool                `json:"skipped"`
	SkipReason string              `json:"skipReason,omitempty"`
	Check      PresenceCheckResult `json:"check"`
}

type MindEvent struct {
	DeviceID string
	Type     string
	Summary  string
	Payload  map[string]any
}

type MindSink interface {
	IngestMindEvent(ctx context.Context, event MindEvent) error
}
