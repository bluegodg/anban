package vision

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func TestServiceCaptureCallsDeviceMCPTool(t *testing.T) {
	xc := &visionClient{
		raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg","presence":"someone"}`),
	}
	svc := NewService(xc)

	result, err := svc.Capture(context.Background(), CaptureRequest{
		DeviceID: " dev-001 ",
		Tool:     "camera.capture",
		Args:     map[string]any{"quality": "low"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if xc.gotDeviceID != "dev-001" {
		t.Fatalf("deviceID = %q, want trimmed dev-001", xc.gotDeviceID)
	}
	if xc.gotTool != "camera.capture" {
		t.Fatalf("tool = %q, want camera.capture", xc.gotTool)
	}
	if xc.gotArgs["quality"] != "low" {
		t.Fatalf("args = %+v, want quality low", xc.gotArgs)
	}
	if result.DeviceID != "dev-001" || result.Tool != "camera.capture" {
		t.Fatalf("result = %+v, want dev-001 camera.capture", result)
	}
	if string(result.Raw) != `{"imageUrl":"https://example.test/capture.jpg","presence":"someone"}` {
		t.Fatalf("raw = %s", result.Raw)
	}
}

func TestServiceCaptureUsesDefaultTool(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)

	result, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if xc.gotTool != DefaultCaptureTool {
		t.Fatalf("tool = %q, want default %q", xc.gotTool, DefaultCaptureTool)
	}
	if result.Tool != DefaultCaptureTool {
		t.Fatalf("result tool = %q, want default %q", result.Tool, DefaultCaptureTool)
	}
}

func TestServiceCaptureBoundsMCPCallForPRDLatency(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"presence":"someone"}`)}
	svc := NewService(xc)

	if _, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !xc.gotDeadline {
		t.Fatal("MCP call context has no deadline; PRD #7 requires vision trigger latency <= 8s")
	}
	remaining := time.Until(xc.deadline)
	if remaining <= 0 || remaining > 8*time.Second {
		t.Fatalf("MCP call deadline remaining = %s, want within 8s", remaining)
	}
}

func TestServiceCaptureRejectsMissingDeviceID(t *testing.T) {
	svc := NewService(&visionClient{})

	_, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCaptureAndObservePresenceUsesMCPPresenceSignal(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/empty.jpg","presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{result: sharedtypes.ProactiveGreetingResult{Status: "played", Text: "王阿姨，回来啦"}}
	svc := NewService(xc, trigger)
	ctx := context.Background()

	empty, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("CaptureAndObservePresence no_one: %v", err)
	}
	if empty.Observation.Presence != PresenceNoOne || empty.Observation.TriggeredGreeting {
		t.Fatalf("empty observation = %+v, want no_one without greeting", empty.Observation)
	}
	if string(empty.Capture.Raw) != `{"imageUrl":"https://example.test/empty.jpg","presence":"no_one"}` {
		t.Fatalf("capture raw = %s", empty.Capture.Raw)
	}

	xc.raw = json.RawMessage(`{"imageUrl":"https://example.test/return.jpg","presence":"someone"}`)
	returned, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("CaptureAndObservePresence someone: %v", err)
	}
	if !returned.Observation.TriggeredGreeting {
		t.Fatalf("returned observation = %+v, want greeting triggered", returned.Observation)
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one return greeting", trigger.calls)
	}
}

func TestServiceCaptureAndObservePresenceBoundsWholePRDLatency(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{}
	svc := NewService(xc, trigger)
	ctx := context.Background()

	if _, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("CaptureAndObservePresence no_one: %v", err)
	}
	xc.raw = json.RawMessage(`{"presence":"someone"}`)
	if _, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("CaptureAndObservePresence someone: %v", err)
	}

	if !trigger.gotDeadline {
		t.Fatal("greeting trigger context has no deadline; PRD #7 requires capture plus trigger latency <= 8s")
	}
	remaining := time.Until(trigger.deadline)
	if remaining <= 0 || remaining > 8*time.Second {
		t.Fatalf("greeting trigger deadline remaining = %s, want within the shared 8s vision budget", remaining)
	}
}

func TestServiceCaptureAndObservePresenceRequiresPresenceSignal(t *testing.T) {
	tests := []json.RawMessage{
		json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`),
		json.RawMessage(`{"presence":"maybe"}`),
		json.RawMessage(`not-json`),
	}
	for _, raw := range tests {
		t.Run(string(raw), func(t *testing.T) {
			svc := NewService(&visionClient{raw: raw}, &fakeGreetingTrigger{})
			_, err := svc.CaptureAndObservePresence(context.Background(), CaptureRequest{DeviceID: "dev-001"})
			if !errors.Is(err, ErrPresenceUnavailable) {
				t.Fatalf("err = %v, want ErrPresenceUnavailable", err)
			}
		})
	}
}

func TestServicePollPresenceSkipsOfflineDevice(t *testing.T) {
	xc := &visionClient{
		status: xiaozhiclient.DeviceStatus{DeviceID: "dev-001", Online: false},
		raw:    json.RawMessage(`{"presence":"someone"}`),
	}
	svc := NewService(xc, &fakeGreetingTrigger{})

	result, err := svc.PollPresence(context.Background(), " dev-001 ")
	if err != nil {
		t.Fatalf("PollPresence offline: %v", err)
	}
	if !result.Skipped || result.SkipReason != "device offline" {
		t.Fatalf("result = %+v, want skipped device offline", result)
	}
	if result.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want trimmed dev-001", result.DeviceID)
	}
	if xc.statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want one status check", xc.statusCalls)
	}
	if xc.mcpCalls != 0 {
		t.Fatalf("mcpCalls = %d, want no camera call for offline device", xc.mcpCalls)
	}
}

func TestServicePollPresenceCapturesOnlineDevice(t *testing.T) {
	xc := &visionClient{
		status: xiaozhiclient.DeviceStatus{DeviceID: "dev-001", Online: true},
		raw:    json.RawMessage(`{"presence":"no_one"}`),
	}
	svc := NewService(xc, &fakeGreetingTrigger{})

	result, err := svc.PollPresence(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("PollPresence online: %v", err)
	}
	if result.Skipped {
		t.Fatalf("result = %+v, want online device captured", result)
	}
	if result.Check.Observation.Presence != PresenceNoOne {
		t.Fatalf("presence = %q, want no_one", result.Check.Observation.Presence)
	}
	if xc.statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want one status check", xc.statusCalls)
	}
	if xc.mcpCalls != 1 || xc.gotTool != DefaultCaptureTool {
		t.Fatalf("mcpCalls = %d tool=%q, want one default camera call", xc.mcpCalls, xc.gotTool)
	}
}

func TestServiceObservePresenceTriggersGreetingWhenSomeoneReturns(t *testing.T) {
	trigger := &fakeGreetingTrigger{
		result: sharedtypes.ProactiveGreetingResult{
			Status: "played",
			Text:   "王阿姨，回来啦，今天过得怎么样？",
		},
	}
	svc := NewService(&visionClient{}, trigger)
	ctx := context.Background()

	first, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: " dev-001 ", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence first someone: %v", err)
	}
	if first.TriggeredGreeting {
		t.Fatalf("first observation triggered greeting, want no startup greeting: %+v", first)
	}

	left, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceNoOne})
	if err != nil {
		t.Fatalf("ObservePresence no one: %v", err)
	}
	if left.PreviousPresence != PresenceSomeone || left.Presence != PresenceNoOne {
		t.Fatalf("left result = %+v, want someone -> no_one", left)
	}

	returned, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence returned: %v", err)
	}
	if !returned.TriggeredGreeting {
		t.Fatalf("returned result = %+v, want greeting triggered", returned)
	}
	if returned.PreviousPresence != PresenceNoOne || returned.Presence != PresenceSomeone {
		t.Fatalf("returned result = %+v, want no_one -> someone", returned)
	}
	if returned.Greeting == nil || returned.Greeting.Status != "played" {
		t.Fatalf("greeting = %+v, want played greeting", returned.Greeting)
	}
	if trigger.calls != 1 || trigger.deviceIDs[0] != "dev-001" {
		t.Fatalf("trigger calls = %d deviceIDs=%+v, want one trimmed dev-001", trigger.calls, trigger.deviceIDs)
	}
}

func TestServiceObservePresenceDoesNotRepeatGreetingWhileStillSomeone(t *testing.T) {
	trigger := &fakeGreetingTrigger{}
	svc := NewService(&visionClient{}, trigger)
	ctx := context.Background()

	_, _ = svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceNoOne})
	_, _ = svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	still, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence still someone: %v", err)
	}
	if still.TriggeredGreeting {
		t.Fatalf("still result = %+v, want no repeated greeting", still)
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want only the no_one -> someone transition", trigger.calls)
	}
}

func TestServiceObservePresenceRejectsBadInput(t *testing.T) {
	svc := NewService(&visionClient{}, &fakeGreetingTrigger{})

	tests := []PresenceObservationRequest{
		{Presence: PresenceSomeone},
		{DeviceID: "dev-001", Presence: "maybe"},
	}
	for _, req := range tests {
		if _, err := svc.ObservePresence(context.Background(), req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("ObservePresence(%+v) err = %v, want ErrInvalidInput", req, err)
		}
	}
}

type visionClient struct {
	xiaozhiclient.FakeClient
	raw         json.RawMessage
	err         error
	status      xiaozhiclient.DeviceStatus
	statusErr   error
	statusCalls int
	mcpCalls    int
	gotDeviceID string
	gotTool     string
	gotArgs     map[string]any
	gotDeadline bool
	deadline    time.Time
}

func (c *visionClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	c.statusCalls++
	status := c.status
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}
	return status, c.statusErr
}

func (c *visionClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	c.mcpCalls++
	c.gotDeviceID = deviceID
	c.gotTool = tool
	c.gotArgs = args
	c.deadline, c.gotDeadline = ctx.Deadline()
	return c.raw, c.err
}

type fakeGreetingTrigger struct {
	result      sharedtypes.ProactiveGreetingResult
	err         error
	calls       int
	deviceIDs   []string
	gotDeadline bool
	deadline    time.Time
}

func (f *fakeGreetingTrigger) TriggerProactiveGreeting(ctx context.Context, deviceID string) (sharedtypes.ProactiveGreetingResult, error) {
	f.calls++
	f.deviceIDs = append(f.deviceIDs, deviceID)
	f.deadline, f.gotDeadline = ctx.Deadline()
	return f.result, f.err
}
