package vision

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const visionMCPTimeout = 8 * time.Second

type Service struct {
	xc               xiaozhiclient.Client
	greetingTrigger  sharedtypes.ProactiveGreetingTrigger
	mu               sync.Mutex
	presenceByDevice map[string]Presence
}

func NewService(xc xiaozhiclient.Client, triggers ...sharedtypes.ProactiveGreetingTrigger) *Service {
	var trigger sharedtypes.ProactiveGreetingTrigger
	if len(triggers) > 0 {
		trigger = triggers[0]
	}
	return &Service{
		xc:               xc,
		greetingTrigger:  trigger,
		presenceByDevice: make(map[string]Presence),
	}
}

func (s *Service) Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return CaptureResult{}, ErrInvalidInput
	}

	tool := strings.TrimSpace(req.Tool)
	if tool == "" {
		tool = DefaultCaptureTool
	}

	callCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	raw, err := s.xc.CallDeviceMCPTool(callCtx, deviceID, tool, req.Args)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		DeviceID: deviceID,
		Tool:     tool,
		Raw:      raw,
	}, nil
}

func (s *Service) CaptureAndObservePresence(ctx context.Context, req CaptureRequest) (PresenceCheckResult, error) {
	workflowCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	capture, err := s.Capture(workflowCtx, req)
	if err != nil {
		return PresenceCheckResult{}, err
	}

	presence, err := parsePresence(capture.Raw)
	if err != nil {
		return PresenceCheckResult{Capture: capture}, err
	}
	observation, err := s.ObservePresence(workflowCtx, PresenceObservationRequest{
		DeviceID: capture.DeviceID,
		Presence: presence,
	})
	return PresenceCheckResult{
		Capture:     capture,
		Observation: observation,
	}, err
}

func (s *Service) PollPresence(ctx context.Context, deviceID string) (PresencePollResult, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return PresencePollResult{}, ErrInvalidInput
	}

	workflowCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	result := PresencePollResult{DeviceID: deviceID}
	status, err := s.xc.GetDeviceStatus(workflowCtx, deviceID)
	if err != nil {
		return result, err
	}
	if !status.Online {
		result.Skipped = true
		result.SkipReason = "device offline"
		return result, nil
	}

	check, err := s.CaptureAndObservePresence(workflowCtx, CaptureRequest{DeviceID: deviceID})
	result.Check = check
	return result, err
}

func (s *Service) ObservePresence(ctx context.Context, req PresenceObservationRequest) (PresenceObservationResult, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	presence := normalizePresence(req.Presence)
	if deviceID == "" || presence == PresenceUnknown {
		return PresenceObservationResult{}, ErrInvalidInput
	}

	observedAt := req.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	s.mu.Lock()
	previous := s.presenceByDevice[deviceID]
	if previous == "" {
		previous = PresenceUnknown
	}
	s.presenceByDevice[deviceID] = presence
	shouldTrigger := previous == PresenceNoOne && presence == PresenceSomeone
	s.mu.Unlock()

	result := PresenceObservationResult{
		DeviceID:         deviceID,
		PreviousPresence: previous,
		Presence:         presence,
		ObservedAt:       observedAt,
	}
	if !shouldTrigger || s.greetingTrigger == nil {
		return result, nil
	}

	greeting, err := s.greetingTrigger.TriggerProactiveGreeting(ctx, deviceID)
	result.Greeting = &greeting
	if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.TriggeredGreeting = true
	return result, nil
}

func withVisionMCPTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= visionMCPTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, visionMCPTimeout)
}

func parsePresence(raw json.RawMessage) (Presence, error) {
	var payload struct {
		Presence Presence `json:"presence"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return PresenceUnknown, ErrPresenceUnavailable
	}
	presence := normalizePresence(payload.Presence)
	if presence == PresenceUnknown {
		return PresenceUnknown, ErrPresenceUnavailable
	}
	return presence, nil
}

func normalizePresence(presence Presence) Presence {
	switch presence {
	case PresenceSomeone, PresenceNoOne:
		return presence
	default:
		return PresenceUnknown
	}
}
