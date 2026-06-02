package xiaozhiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/pkg/types"
)

// HTTPClient 是 Client 的真实现：对 manager 的 /api/open/v1。
type HTTPClient struct {
	baseURL string
	token   string
	hc      *http.Client
}

func NewHTTPClient(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
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

type rolePromptReq struct {
	Prompt string `json:"prompt"`
}

type historyMessagePayload struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Text      string `json:"text"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
	At        string `json:"at"`
	Timestamp string `json:"timestamp"`
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

func (c *HTTPClient) GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/open/v1/devices/"+url.PathEscape(deviceID), nil)
	if err != nil {
		return DeviceStatus{}, err
	}

	status, err := decodeDeviceStatus(body)
	if err != nil {
		return DeviceStatus{}, err
	}
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}
	return status, nil
}

func (c *HTTPClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	body, err := json.Marshal(rolePromptReq{Prompt: prompt})
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPut, "/api/open/v1/devices/"+url.PathEscape(deviceID)+"/role-prompt", body)
	return err
}

func (c *HTTPClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error) {
	q := url.Values{}
	if deviceID != "" {
		q.Set("deviceId", deviceID)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
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
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("xiaozhi manager %s %s -> %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

type deviceStatusEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type deviceStatusPayload struct {
	DeviceID          string `json:"device_id"`
	ID                string `json:"id"`
	Online            *bool  `json:"online"`
	Status            string `json:"status"`
	LastActiveAt      string `json:"last_active_at"`
	LastSeenAt        string `json:"last_seen_at"`
	LastInteractionAt string `json:"last_interaction_at"`
}

func decodeDeviceStatus(body []byte) (DeviceStatus, error) {
	raw := body
	var envelope deviceStatusEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		raw = envelope.Data
	}

	var payload deviceStatusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return DeviceStatus{}, err
	}

	lastActive, err := parseOptionalTime(firstNonEmpty(payload.LastActiveAt, payload.LastSeenAt, payload.LastInteractionAt))
	if err != nil {
		return DeviceStatus{}, err
	}

	online := false
	if payload.Online != nil {
		online = *payload.Online
	} else {
		online = strings.EqualFold(payload.Status, "online") || strings.EqualFold(payload.Status, "active")
		if !online && !lastActive.IsZero() {
			online = time.Since(lastActive) <= 30*time.Second
		}
	}

	deviceID := payload.DeviceID
	if deviceID == "" {
		deviceID = payload.ID
	}

	return DeviceStatus{
		DeviceID:     deviceID,
		Online:       online,
		LastActiveAt: lastActive,
	}, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
		var list struct {
			Messages []historyMessagePayload `json:"messages"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		payloads = list.Messages
	}

	messages := make([]HistoryMessage, 0, len(payloads))
	for _, payload := range payloads {
		at, err := parseOptionalTime(firstNonEmpty(payload.CreatedAt, payload.At, payload.Timestamp))
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

// CallDeviceMCPTool 在 vision 域 follow-on 计划中据真实端点补齐。
func (c *HTTPClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return nil, types.ErrNotImplemented
}
