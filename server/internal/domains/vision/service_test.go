package vision

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
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

func TestServiceCaptureRejectsMissingDeviceID(t *testing.T) {
	svc := NewService(&visionClient{})

	_, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

type visionClient struct {
	xiaozhiclient.FakeClient
	raw         json.RawMessage
	err         error
	gotDeviceID string
	gotTool     string
	gotArgs     map[string]any
}

func (c *visionClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	c.gotDeviceID = deviceID
	c.gotTool = tool
	c.gotArgs = args
	return c.raw, c.err
}
