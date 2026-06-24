package status

import (
	"context"
	"encoding/json"
	"strings"
)

const (
	mcpToolDeviceStatus = "self_get_device_status"
	mcpToolSetVolume    = "self_audio_speaker_set_volume"
	minVolume           = 0
	maxVolume           = 100
)

// GetDevicePanel returns a settings-facing snapshot of the bound device:
// device code + online + (when online) live volume / screen / network read via
// the device's self_get_device_status MCP tool. The device code is always
// available from the manager; the MCP-sourced fields are best-effort and stay
// nil when the device is offline or does not report them (e.g. battery).
func (s *Service) GetDevicePanel(ctx context.Context, deviceID string) (DevicePanel, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DevicePanel{}, ErrInvalidInput
	}
	status, err := s.xc.GetDeviceStatus(ctx, deviceID)
	if err != nil {
		return DevicePanel{}, err
	}
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}
	panel := DevicePanel{
		DeviceID:     status.DeviceID,
		DeviceCode:   status.DeviceCode,
		Online:       status.Online,
		LastActiveAt: timePtr(status.LastActiveAt),
	}
	// The manager's last-active "online" flag goes stale while the device is
	// MQTT-connected but idle; the MCP control plane stays reachable (same as the
	// camera). So always attempt the live read and treat a successful MCP call as
	// the real online/controllable signal. A truly offline device fails this fast
	// (its MCP tool list is empty), so it does not hang.
	raw, err := s.xc.CallDeviceMCPTool(ctx, deviceID, mcpToolDeviceStatus, nil)
	if err != nil {
		return panel, nil
	}
	if st, ok := parseDeviceStatusMCP(raw); ok {
		panel.Online = true
		applyDeviceStatus(&panel, st)
	}
	return panel, nil
}

// SetVolume sets the device speaker volume (0..100) via MCP control-plane.
func (s *Service) SetVolume(ctx context.Context, req SetVolumeRequest) error {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" || req.Volume < minVolume || req.Volume > maxVolume {
		return ErrInvalidInput
	}
	_, err := s.xc.CallDeviceMCPTool(ctx, deviceID, mcpToolSetVolume, map[string]any{"volume": req.Volume})
	return err
}

type deviceStatusMCP struct {
	AudioSpeaker *struct {
		Volume *int `json:"volume"`
	} `json:"audio_speaker"`
	Screen *struct {
		Brightness *int   `json:"brightness"`
		Theme      string `json:"theme"`
	} `json:"screen"`
	Network *struct {
		Type   string `json:"type"`
		SSID   string `json:"ssid"`
		Signal string `json:"signal"`
	} `json:"network"`
	Battery *struct {
		Level *int `json:"level"`
	} `json:"battery"`
}

func applyDeviceStatus(panel *DevicePanel, st deviceStatusMCP) {
	if st.AudioSpeaker != nil {
		panel.Volume = st.AudioSpeaker.Volume
	}
	if st.Screen != nil {
		panel.Brightness = st.Screen.Brightness
		panel.Theme = strings.TrimSpace(st.Screen.Theme)
	}
	if st.Network != nil {
		panel.Network = &DeviceNetwork{
			Type:   strings.TrimSpace(st.Network.Type),
			SSID:   strings.TrimSpace(st.Network.SSID),
			Signal: strings.TrimSpace(st.Network.Signal),
		}
	}
	if st.Battery != nil {
		panel.Battery = st.Battery.Level
	}
}

// parseDeviceStatusMCP unwraps the manager mcp-call envelope. The status JSON is
// triple-nested: data.result is a JSON string {"content":[{"text":"<status json>"}]},
// and that text is itself the JSON status payload.
func parseDeviceStatusMCP(raw json.RawMessage) (deviceStatusMCP, bool) {
	statusJSON, ok := extractMCPText(raw)
	if !ok {
		return deviceStatusMCP{}, false
	}
	var st deviceStatusMCP
	if err := json.Unmarshal([]byte(statusJSON), &st); err != nil {
		return deviceStatusMCP{}, false
	}
	return st, true
}

func extractMCPText(raw json.RawMessage) (string, bool) {
	inner := []byte(raw)
	// Layer 1: manager mcp-call result wrapper {"result":"<json string>", ...}.
	var env struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && strings.TrimSpace(env.Result) != "" {
		inner = []byte(env.Result)
	}
	// Layer 2: MCP content wrapper {"content":[{"text":"<status json>"}]}.
	var wrap struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(inner, &wrap); err == nil && len(wrap.Content) > 0 && strings.TrimSpace(wrap.Content[0].Text) != "" {
		return wrap.Content[0].Text, true
	}
	// Fallback: inner may already be the status payload.
	if strings.Contains(string(inner), "audio_speaker") {
		return string(inner), true
	}
	return "", false
}
