package status

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type panelClient struct {
	xiaozhiclient.FakeClient
	status    xiaozhiclient.DeviceStatus
	statusErr error
	mcpResult json.RawMessage
	mcpErr    error
	gotTool   string
	gotArgs   map[string]any
	mcpCalls  int
}

func (c *panelClient) GetDeviceStatus(_ context.Context, _ string) (xiaozhiclient.DeviceStatus, error) {
	return c.status, c.statusErr
}

func (c *panelClient) CallDeviceMCPTool(_ context.Context, _ string, tool string, args map[string]any) (json.RawMessage, error) {
	c.mcpCalls++
	c.gotTool = tool
	c.gotArgs = args
	return c.mcpResult, c.mcpErr
}

func TestGetDevicePanelParsesLiveStatus(t *testing.T) {
	// Exact triple-nested envelope shape returned by manager mcp-call (verified on real device).
	result := `{"agent_id":"","device_id":"dev","result":"{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"audio_speaker\\\":{\\\"volume\\\":100},\\\"screen\\\":{\\\"brightness\\\":75,\\\"theme\\\":\\\"light\\\"},\\\"network\\\":{\\\"type\\\":\\\"wifi\\\",\\\"ssid\\\":\\\"CMCC-4DF9\\\",\\\"signal\\\":\\\"strong\\\"}}\"}]}","tool_name":"self_get_device_status"}`
	xc := &panelClient{
		status:    xiaozhiclient.DeviceStatus{DeviceID: "dev", DeviceCode: "anban1", Online: true},
		mcpResult: json.RawMessage(result),
	}
	panel, err := NewService(xc).GetDevicePanel(context.Background(), " dev ")
	if err != nil {
		t.Fatalf("GetDevicePanel: %v", err)
	}
	if panel.DeviceCode != "anban1" || !panel.Online {
		t.Fatalf("panel = %+v, want anban1 online", panel)
	}
	if panel.Volume == nil || *panel.Volume != 100 {
		t.Fatalf("volume = %v, want 100", panel.Volume)
	}
	if panel.Brightness == nil || *panel.Brightness != 75 || panel.Theme != "light" {
		t.Fatalf("screen = %v/%q, want 75/light", panel.Brightness, panel.Theme)
	}
	if panel.Network == nil || panel.Network.SSID != "CMCC-4DF9" || panel.Network.Signal != "strong" {
		t.Fatalf("network = %+v, want CMCC-4DF9/strong", panel.Network)
	}
	if panel.Battery != nil {
		t.Fatalf("battery = %v, want nil (firmware does not report it)", panel.Battery)
	}
}

func TestGetDevicePanelReachableViaMCPDespiteStaleOnlineFlag(t *testing.T) {
	// Regression: manager last-active "online" is false (stale, device MQTT-idle),
	// but the MCP read succeeds -> the panel must treat it as online/controllable.
	result := `{"result":"{\"content\":[{\"text\":\"{\\\"audio_speaker\\\":{\\\"volume\\\":50}}\"}]}"}`
	xc := &panelClient{
		status:    xiaozhiclient.DeviceStatus{DeviceID: "dev", DeviceCode: "anban1", Online: false},
		mcpResult: json.RawMessage(result),
	}
	panel, err := NewService(xc).GetDevicePanel(context.Background(), "dev")
	if err != nil {
		t.Fatalf("GetDevicePanel: %v", err)
	}
	if !panel.Online {
		t.Fatalf("panel.Online = false, want true (reachable via MCP)")
	}
	if panel.Volume == nil || *panel.Volume != 50 {
		t.Fatalf("volume = %v, want 50", panel.Volume)
	}
}

func TestGetDevicePanelMCPFailureDegradesGracefully(t *testing.T) {
	xc := &panelClient{
		status: xiaozhiclient.DeviceStatus{DeviceID: "dev", DeviceCode: "anban1", Online: false},
		mcpErr: errors.New("device offline"),
	}
	panel, err := NewService(xc).GetDevicePanel(context.Background(), "dev")
	if err != nil {
		t.Fatalf("GetDevicePanel: %v", err)
	}
	if panel.DeviceCode != "anban1" || panel.Online {
		t.Fatalf("panel = %+v, want anban1 offline", panel)
	}
	if panel.Volume != nil || panel.Network != nil {
		t.Fatalf("unreachable panel should have no live fields: %+v", panel)
	}
}

func TestSetVolumeValidatesRange(t *testing.T) {
	xc := &panelClient{}
	svc := NewService(xc)
	for _, v := range []int{-1, 101} {
		if err := svc.SetVolume(context.Background(), SetVolumeRequest{DeviceID: "dev", Volume: v}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("volume %d: err = %v, want ErrInvalidInput", v, err)
		}
	}
	if xc.mcpCalls != 0 {
		t.Fatalf("mcpCalls = %d, want 0 for invalid volume", xc.mcpCalls)
	}
}

func TestSetVolumeCallsMCPTool(t *testing.T) {
	xc := &panelClient{}
	if err := NewService(xc).SetVolume(context.Background(), SetVolumeRequest{DeviceID: " dev ", Volume: 40}); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if xc.gotTool != "self_audio_speaker_set_volume" {
		t.Fatalf("tool = %q, want self_audio_speaker_set_volume", xc.gotTool)
	}
	if v, _ := xc.gotArgs["volume"].(int); v != 40 {
		t.Fatalf("args = %v, want volume 40", xc.gotArgs)
	}
}
