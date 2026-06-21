package xiaozhiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient 是 Client 的真实现：对 manager 的 /api/open/v1。
type HTTPClient struct {
	baseURL string
	token   string
	hc      *http.Client
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

// 确保 *HTTPClient 实现了 Client 接口（编译期检查）。
var _ Client = (*HTTPClient)(nil)

type injectReq struct {
	DeviceID   string `json:"device_id"`
	Message    string `json:"message"`
	SkipLlm    bool   `json:"skip_llm"`
	AutoListen *bool  `json:"auto_listen,omitempty"`
}

type mcpCallReq struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type managerDevicePayload struct {
	ID                json.RawMessage `json:"id"`
	DeviceID          string          `json:"device_id"`
	DeviceName        string          `json:"device_name"`
	AgentID           json.RawMessage `json:"agent_id"`
	Online            *bool           `json:"online"`
	Status            string          `json:"status"`
	LastActiveAt      json.RawMessage `json:"last_active_at"`
	LastSeenAt        json.RawMessage `json:"last_seen_at"`
	LastInteractionAt json.RawMessage `json:"last_interaction_at"`
}

type mcpToolPayload struct {
	Name     string `json:"name"`
	ToolName string `json:"tool_name"`
}

type historyMessagePayload struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Text      string          `json:"text"`
	Message   string          `json:"message"`
	CreatedAt json.RawMessage `json:"created_at"`
	At        json.RawMessage `json:"at"`
	Timestamp json.RawMessage `json:"timestamp"`
}

func (c *HTTPClient) InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error {
	body, err := json.Marshal(injectReq{
		DeviceID:   deviceID,
		Message:    text,
		SkipLlm:    opts.SkipLLM,
		AutoListen: opts.AutoListen,
	})
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPost, "/api/open/v1/devices/inject-message", body)
	return err
}

func (c *HTTPClient) CheckManagerAccess(ctx context.Context) error {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices", nil)
	if err != nil {
		return err
	}
	_, err = decodeManagerDevices(body)
	return err
}

func (c *HTTPClient) GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices", nil)
	if err != nil {
		return DeviceStatus{}, err
	}
	devices, err := decodeManagerDevices(body)
	if err != nil {
		return DeviceStatus{}, err
	}

	target := strings.TrimSpace(deviceID)
	for _, device := range devices {
		if !device.matches(target) {
			continue
		}
		status, err := device.toDeviceStatus(target)
		if err != nil {
			return DeviceStatus{}, err
		}
		return status, nil
	}
	return DeviceStatus{}, fmt.Errorf("xiaozhi manager device %q not found", deviceID)
}

func (c *HTTPClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if containsCompanionContext(prompt) {
		return ErrCompanionContextInStylePrompt
	}
	agentID, err := c.findAgentIDForDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/agents/"+url.PathEscape(agentID), nil)
	if err != nil {
		return err
	}
	agent, err := decodeManagerAgent(body)
	if err != nil {
		return err
	}
	agent["custom_prompt"] = prompt

	updateBody, err := json.Marshal(agent)
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPut, "/api/open/v1/agents/"+url.PathEscape(agentID), updateBody)
	return err
}

const (
	anbanContextBeginMarker = "<!-- ANBAN_CONTEXT_BEGIN -->"
	anbanContextEndMarker   = "<!-- ANBAN_CONTEXT_END -->"
)

func containsCompanionContext(prompt string) bool {
	for _, marker := range []string{
		anbanContextBeginMarker,
		anbanContextEndMarker,
		"老人本名：",
		"陪伴对象姓名：",
		"常用称呼：",
		"近期记忆：",
		"专属记忆：",
		"心境：",
		"心智上下文：",
	} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

func (c *HTTPClient) findAgentIDForDevice(ctx context.Context, deviceID string) (string, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices", nil)
	if err != nil {
		return "", err
	}
	devices, err := decodeManagerDevices(body)
	if err != nil {
		return "", err
	}

	target := strings.TrimSpace(deviceID)
	for _, device := range devices {
		if !device.matches(target) {
			continue
		}
		agentID := rawJSONIDString(device.AgentID)
		if agentID == "" || agentID == "0" {
			return "", fmt.Errorf("xiaozhi manager device %q has no bound agent", deviceID)
		}
		return agentID, nil
	}
	return "", fmt.Errorf("xiaozhi manager device %q not found", deviceID)
}

func (c *HTTPClient) findManagerDeviceID(ctx context.Context, deviceID string) (string, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices", nil)
	if err != nil {
		return "", err
	}
	devices, err := decodeManagerDevices(body)
	if err != nil {
		return "", err
	}

	target := strings.TrimSpace(deviceID)
	for _, device := range devices {
		if !device.matches(target) {
			continue
		}
		if device.explicitlyOffline() {
			return "", fmt.Errorf("%w: %s", ErrDeviceOffline, deviceID)
		}
		managerID := rawJSONIDString(device.ID)
		if managerID == "" || managerID == "0" {
			return "", fmt.Errorf("xiaozhi manager device %q has no manager id", deviceID)
		}
		return managerID, nil
	}
	return "", fmt.Errorf("xiaozhi manager device %q not found", deviceID)
}

func (c *HTTPClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error) {
	q := url.Values{}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	if limit > 0 {
		q.Set("page_size", strconv.Itoa(limit))
	}
	path := "/api/open/v1/history/messages"
	if query := q.Encode(); query != "" {
		path += "?" + query
	}

	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeHistoryMessages(body)
}

// do 发一个带 X-API-Token 的请求；2xx 返回响应体，否则返回错误（含状态码与响应片段）。
func (c *HTTPClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", c.token)

	resp, err := c.hc.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return nil, fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xiaozhi manager %s %s -> %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (d managerDevicePayload) matches(deviceID string) bool {
	candidates := []string{
		d.DeviceID,
		d.DeviceName,
		rawJSONIDString(d.ID),
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == deviceID {
			return true
		}
	}
	return false
}

func (d managerDevicePayload) toDeviceStatus(fallbackDeviceID string) (DeviceStatus, error) {
	lastActive, err := parseOptionalRawTime(d.LastActiveAt, d.LastSeenAt, d.LastInteractionAt)
	if err != nil {
		return DeviceStatus{}, err
	}

	online := false
	if d.Online != nil {
		online = *d.Online
	} else {
		online = strings.EqualFold(d.Status, "online") || strings.EqualFold(d.Status, "active")
		if !online && !lastActive.IsZero() {
			online = time.Since(lastActive) <= 30*time.Second
		}
	}

	deviceID := firstNonEmpty(d.DeviceID, d.DeviceName, rawJSONIDString(d.ID), fallbackDeviceID)
	return DeviceStatus{
		DeviceID:     deviceID,
		Online:       online,
		LastActiveAt: lastActive,
	}, nil
}

func (d managerDevicePayload) explicitlyOffline() bool {
	if d.Online != nil {
		return !*d.Online
	}
	status := strings.ToLower(strings.TrimSpace(d.Status))
	switch status {
	case "offline", "inactive", "disconnected":
		return true
	default:
		return false
	}
}

func decodeManagerDevices(body []byte) ([]managerDevicePayload, error) {
	raw := unwrapData(body)
	// 设备表为空时 manager 会返回 null / {"data":null} / {"data":[]}，统一当作空列表，而非报错。
	if isJSONNullOrEmpty(raw) {
		return nil, nil
	}
	if devices, err := unmarshalManagerDeviceArray(raw); err == nil {
		return devices, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if dataField, ok := object["data"]; ok && isJSONNullOrEmpty(dataField) {
		return nil, nil
	}
	for _, key := range []string{"devices", "items", "list", "records", "rows"} {
		nested, ok := object[key]
		if !ok || len(nested) == 0 {
			continue
		}
		if isJSONNullOrEmpty(nested) {
			return nil, nil
		}
		if devices, err := unmarshalManagerDeviceArray(nested); err == nil {
			return devices, nil
		}
	}
	return nil, fmt.Errorf("xiaozhi manager devices response does not contain a device list")
}

func unmarshalManagerDeviceArray(raw json.RawMessage) ([]managerDevicePayload, error) {
	var devices []managerDevicePayload
	if err := json.Unmarshal(raw, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// isJSONNullOrEmpty 判断一段原始 JSON 是否为空或显式 null。
func isJSONNullOrEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}

func decodeManagerAgent(body []byte) (map[string]any, error) {
	raw := unwrapData(body)
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		for _, key := range []string{"agent", "item"} {
			nested, ok := object[key]
			if !ok || len(nested) == 0 || string(nested) == "null" {
				continue
			}
			return unmarshalObjectMap(nested)
		}
	}
	return unmarshalObjectMap(raw)
}

func unmarshalObjectMap(raw json.RawMessage) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if len(object) == 0 {
		return nil, fmt.Errorf("xiaozhi manager response is not an object")
	}
	return object, nil
}

func rawJSONIDString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		return unixTimestampToTime(unix), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseOptionalRawTime(values ...json.RawMessage) (time.Time, error) {
	raw := firstNonEmptyRaw(values...)
	if isJSONNullOrEmpty(raw) {
		return time.Time{}, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return parseOptionalTime(value)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return parseOptionalTime(number.String())
	}
	return time.Time{}, fmt.Errorf("manager time value is not a string or number")
}

func unixTimestampToTime(value int64) time.Time {
	const millisThreshold = int64(1_000_000_000_000)
	if value >= millisThreshold || value <= -millisThreshold {
		sec := value / 1000
		nsec := (value % 1000) * int64(time.Millisecond)
		return time.Unix(sec, nsec).UTC()
	}
	return time.Unix(value, 0).UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if !isJSONNullOrEmpty(value) {
			return value
		}
	}
	return nil
}

func decodeHistoryMessages(body []byte) ([]HistoryMessage, error) {
	raw := body
	var envelope struct {
		Data     json.RawMessage `json:"data"`
		Messages json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			raw = envelope.Data
		} else if len(envelope.Messages) > 0 && string(envelope.Messages) != "null" {
			raw = envelope.Messages
		}
	}

	var payloads []historyMessagePayload
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(raw, &payloads); err != nil {
			return nil, err
		}
	} else {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, err
		}
		foundList := false
		for _, key := range []string{"messages", "items", "list", "records", "rows"} {
			nested, ok := object[key]
			if !ok || len(nested) == 0 || string(nested) == "null" {
				continue
			}
			foundList = true
			if err := json.Unmarshal(nested, &payloads); err != nil {
				return nil, err
			}
			break
		}
		if !foundList {
			return nil, fmt.Errorf("xiaozhi manager history response does not contain a message list")
		}
	}

	messages := make([]HistoryMessage, 0, len(payloads))
	for _, payload := range payloads {
		at, err := parseOptionalRawTime(payload.CreatedAt, payload.At, payload.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, HistoryMessage{
			Role: strings.TrimSpace(payload.Role),
			Text: firstNonEmpty(payload.Content, payload.Text, payload.Message),
			At:   at,
		})
	}
	return messages, nil
}

func (c *HTTPClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	managerID, err := c.findManagerDeviceID(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	managerToolName, err := c.resolveMCPToolName(ctx, managerID, tool)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(mcpCallReq{ToolName: managerToolName, Arguments: args})
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/api/open/v1/devices/"+url.PathEscape(managerID)+"/mcp-call", body)
	if err != nil {
		return nil, err
	}
	return unwrapData(resp), nil
}

func (c *HTTPClient) resolveMCPToolName(ctx context.Context, managerID, tool string) (string, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices/"+url.PathEscape(managerID)+"/mcp-tools", nil)
	if err != nil {
		return "", err
	}
	tools, err := decodeMCPTools(body)
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(tool)
	for _, item := range tools {
		if resolved, ok := item.resolve(target); ok {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrMCPToolUnavailable, tool)
}

func (t mcpToolPayload) resolve(tool string) (string, bool) {
	target := strings.TrimSpace(tool)
	normalizedTarget := normalizeMCPToolName(target)
	for _, candidate := range []string{t.Name, t.ToolName} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == target || normalizeMCPToolName(candidate) == normalizedTarget {
			return candidate, true
		}
	}
	return "", false
}

func normalizeMCPToolName(name string) string {
	var normalized strings.Builder
	for _, char := range strings.TrimSpace(name) {
		if char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' ||
			char == '_' || char == '-' {
			normalized.WriteRune(char)
			continue
		}
		normalized.WriteByte('_')
	}
	return normalized.String()
}

func decodeMCPTools(body []byte) ([]mcpToolPayload, error) {
	raw := unwrapData(body)
	if isJSONNullOrEmpty(raw) {
		return nil, nil
	}
	if tools, err := unmarshalMCPToolArray(raw); err == nil {
		return tools, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	for _, key := range []string{"tools", "items", "list", "records", "rows"} {
		nested, ok := object[key]
		if !ok || len(nested) == 0 || isJSONNullOrEmpty(nested) {
			continue
		}
		if tools, err := unmarshalMCPToolArray(nested); err == nil {
			return tools, nil
		}
	}
	return nil, fmt.Errorf("xiaozhi manager mcp-tools response does not contain a tool list")
}

func unmarshalMCPToolArray(raw json.RawMessage) ([]mcpToolPayload, error) {
	var tools []mcpToolPayload
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

func unwrapData(body []byte) json.RawMessage {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return envelope.Data
	}
	return json.RawMessage(body)
}
