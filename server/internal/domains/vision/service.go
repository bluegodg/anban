package vision

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

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

	raw, err := s.xc.CallDeviceMCPTool(ctx, deviceID, tool, req.Args)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		DeviceID: deviceID,
		Tool:     tool,
		Raw:      raw,
	}, nil
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

func normalizePresence(presence Presence) Presence {
	switch presence {
	case PresenceSomeone, PresenceNoOne:
		return presence
	default:
		return PresenceUnknown
	}
}
