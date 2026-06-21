package xiaozhiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInjectSpeakSendsCorrectRequest(t *testing.T) {
	var gotPath, gotToken string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"ok"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	skip := false
	auto := true
	err := c.InjectSpeak(context.Background(), "dev-001", "妈，该吃药了", InjectOptions{SkipLLM: skip, AutoListen: &auto})
	if err != nil {
		t.Fatalf("InjectSpeak: %v", err)
	}
	if gotPath != "/api/open/v1/devices/inject-message" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if gotBody["device_id"] != "dev-001" || gotBody["message"] != "妈，该吃药了" {
		t.Fatalf("body = %v", gotBody)
	}
	if gotBody["auto_listen"] != true {
		t.Fatalf("auto_listen = %v, want true", gotBody["auto_listen"])
	}
}

func TestInjectSpeakErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"无效或已过期的API Token"}`))
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL, "bad")
	if err := c.InjectSpeak(context.Background(), "d", "hi", InjectOptions{}); err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

func TestHTTPClientTrimsTrailingSlashFromManagerBaseURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath != "/api/open/v1/devices/inject-message" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL+"/", "tok_abc")
	if err := c.InjectSpeak(context.Background(), "dev-001", "hi", InjectOptions{}); err != nil {
		t.Fatalf("InjectSpeak: %v", err)
	}
	if gotPath != "/api/open/v1/devices/inject-message" {
		t.Fatalf("path = %q, want single-slash API path", gotPath)
	}
}

func TestGetDeviceStatusReadsManagerDeviceList(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	var gotPath, gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		if r.URL.Path != "/api/open/v1/devices" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [
				{
					"id": 1,
					"device_name": "dev-other",
					"online": false,
					"last_active_at": "2026-06-01T08:20:00Z"
				},
				{
					"id": 2,
					"device_name": "dev-001",
					"online": true,
					"last_active_at": "2026-06-01T08:30:00Z"
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	status, err := c.GetDeviceStatus(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	if gotPath != "/api/open/v1/devices" {
		t.Fatalf("path = %q, want device list endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if status.DeviceID != "dev-001" || !status.Online {
		t.Fatalf("status = %+v, want dev-001 online", status)
	}
	if !status.LastActiveAt.Equal(lastActive) {
		t.Fatalf("lastActiveAt = %s, want %s", status.LastActiveAt, lastActive)
	}
}

func TestCheckManagerAccessReadsDevicesEndpoint(t *testing.T) {
	var gotPath, gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if err := c.CheckManagerAccess(context.Background()); err != nil {
		t.Fatalf("CheckManagerAccess: %v", err)
	}
	if gotPath != "/api/open/v1/devices" {
		t.Fatalf("path = %q, want device list endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
}

func TestCheckManagerAccessRejectsMalformedDeviceList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"unexpected":true}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if err := c.CheckManagerAccess(context.Background()); err == nil {
		t.Fatal("expected malformed devices response error, got nil")
	}
}

func TestGetDeviceStatusParsesDirectActivePayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"device_id": "dev-002",
					"status": "active",
					"last_seen_at": "2026-06-01T08:31:00Z"
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	status, err := c.GetDeviceStatus(context.Background(), "dev-002")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	if status.DeviceID != "dev-002" || !status.Online {
		t.Fatalf("status = %+v, want active dev-002", status)
	}
}

func TestGetDeviceStatusRejectsInvalidTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"device_name":"dev-001","last_active_at":"not-a-time"}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if _, err := c.GetDeviceStatus(context.Background(), "dev-001"); err == nil {
		t.Fatal("expected invalid time error, got nil")
	}
}

func TestGetDeviceStatusTrimsManagerTimeFields(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"device_name":"dev-001","last_active_at":"  2026-06-01T08:30:00Z  "}]} `))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	status, err := c.GetDeviceStatus(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	if !status.LastActiveAt.Equal(lastActive) {
		t.Fatalf("lastActiveAt = %s, want %s", status.LastActiveAt, lastActive)
	}
}

func TestGetDeviceStatusParsesUnixNumericLastActiveAt(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"device_name":"dev-001","online":true,"last_active_at":1780302600}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	status, err := c.GetDeviceStatus(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	if !status.LastActiveAt.Equal(lastActive) {
		t.Fatalf("lastActiveAt = %s, want %s", status.LastActiveAt, lastActive)
	}
}

func TestGetDeviceStatusErrorsWhenDeviceIsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"device_name":"dev-other","online":true}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	_, err := c.GetDeviceStatus(context.Background(), "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestSetRolePromptSendsManagerAgentRequest(t *testing.T) {
	var requests []string
	var tokens []string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		tokens = append(tokens, r.Header.Get("X-API-Token"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/open/v1/devices":
			if r.Method != http.MethodGet {
				t.Fatalf("devices method = %q, want GET", r.Method)
			}
			_, _ = w.Write([]byte(`{
				"success": true,
				"data": [
					{"id": 1, "device_name": "dev-other", "agent_id": 7},
					{"id": 2, "device_name": "dev-001", "agent_id": 9}
				]
			}`))
		case "/api/open/v1/agents/9":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{
					"success": true,
					"data": {
						"id": 9,
						"user_id": 5,
						"name": "care-agent",
						"nickname": "小伴",
						"custom_prompt": "old prompt",
						"llm_config_id": "llm-a",
						"tts_config_id": "tts-a",
						"voice": "voice-a",
						"asr_speed": "normal",
						"memory_mode": "short",
						"speaker_chat_mode": "off",
						"mcp_service_names": ["camera"],
						"knowledge_base_ids": [3]
					}
				}`))
			case http.MethodPut:
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				_, _ = w.Write([]byte(`{"success":true}`))
			default:
				t.Fatalf("agent method = %q, want GET or PUT", r.Method)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	stylePrompt := "你是安伴，语气温和、耐心，回答自然简短。"
	err := c.SetRolePrompt(context.Background(), "dev-001", stylePrompt)
	if err != nil {
		t.Fatalf("SetRolePrompt: %v", err)
	}

	wantRequests := []string{
		"GET /api/open/v1/devices",
		"GET /api/open/v1/agents/9",
		"PUT /api/open/v1/agents/9",
	}
	if len(requests) != len(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
	for i, want := range wantRequests {
		if requests[i] != want {
			t.Fatalf("requests = %v, want %v", requests, wantRequests)
		}
	}
	for _, token := range tokens {
		if token != "tok_abc" {
			t.Fatalf("X-API-Token sequence = %v, want tok_abc on every request", tokens)
		}
	}
	gotPrompt, _ := gotBody["custom_prompt"].(string)
	if gotPrompt != stylePrompt {
		t.Fatalf("custom_prompt = %q, want style only %q", gotPrompt, stylePrompt)
	}
	if gotBody["name"] != "care-agent" || gotBody["voice"] != "voice-a" {
		t.Fatalf("body = %v, want existing agent fields preserved", gotBody)
	}
	mcpServices, ok := gotBody["mcp_service_names"].([]any)
	if !ok || len(mcpServices) != 1 || mcpServices[0] != "camera" {
		t.Fatalf("mcp_service_names = %v, want preserved camera", gotBody["mcp_service_names"])
	}
}

func TestSetRolePromptReplacesLegacyManagedContextWithStyleOnly(t *testing.T) {
	var gotBody map[string]any
	srv := newRolePromptServer(t, "说话慢一点，语气亲近。\n\n"+anbanContextBeginMarker+"\n老人本名：王秀英\n近期记忆：王阿姨喜欢豫剧\n"+anbanContextEndMarker+"\n\n回答尽量简短。", &gotBody)
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	stylePrompt := "说话慢一点，语气亲近，回答尽量简短。"
	if err := c.SetRolePrompt(context.Background(), "dev-001", stylePrompt); err != nil {
		t.Fatalf("SetRolePrompt: %v", err)
	}

	gotPrompt, _ := gotBody["custom_prompt"].(string)
	if gotPrompt != stylePrompt {
		t.Fatalf("custom_prompt = %q, want %q", gotPrompt, stylePrompt)
	}
	for _, stale := range []string{"ANBAN_CONTEXT", "王秀英", "王阿姨"} {
		if strings.Contains(gotPrompt, stale) {
			t.Fatalf("custom_prompt = %q, want companion context %q removed", gotPrompt, stale)
		}
	}
}

func TestSetRolePromptRejectsCompanionContext(t *testing.T) {
	var gotBody map[string]any
	srv := newRolePromptServer(t, "old style", &gotBody)
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	err := c.SetRolePrompt(context.Background(), "dev-001", "陪伴对象姓名：蓝\n专属记忆：老人喜欢养花")
	if !errors.Is(err, ErrCompanionContextInStylePrompt) {
		t.Fatalf("err = %v, want ErrCompanionContextInStylePrompt", err)
	}
	if gotBody != nil {
		t.Fatalf("manager update body = %v, want no PUT", gotBody)
	}
}

func newRolePromptServer(t *testing.T, existingPrompt string, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":2,"device_name":"dev-001","agent_id":9}]}`))
		case "/api/open/v1/agents/9":
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": true,
					"data": map[string]any{
						"id":            9,
						"name":          "care-agent",
						"custom_prompt": existingPrompt,
						"voice":         "voice-a",
					},
				})
			case http.MethodPut:
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, gotBody)
				_, _ = w.Write([]byte(`{"success":true}`))
			default:
				t.Fatalf("agent method = %q, want GET or PUT", r.Method)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestSetRolePromptErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"device not found"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if err := c.SetRolePrompt(context.Background(), "missing", "prompt"); err == nil {
		t.Fatal("expected error on non-2xx, got nil")
	}
}

func TestSetRolePromptErrorsWhenDeviceHasNoBoundAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [
				{"id": 2, "device_name": "dev-001", "agent_id": 0}
			]
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	err := c.SetRolePrompt(context.Background(), "dev-001", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no bound agent") {
		t.Fatalf("err = %v, want no bound agent", err)
	}
}

func TestDecodeManagerDevicesParsesNestedListAndStringIDs(t *testing.T) {
	devices, err := decodeManagerDevices([]byte(`{
		"success": true,
		"data": {
			"devices": [
				{"id": "dev-row-1", "device_id": "dev-001", "device_name": "mac-001", "agent_id": "17"}
			]
		}
	}`))
	if err != nil {
		t.Fatalf("decodeManagerDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %+v, want 1", devices)
	}
	if !devices[0].matches("dev-row-1") || !devices[0].matches("dev-001") || !devices[0].matches("mac-001") {
		t.Fatalf("device matches failed for %+v", devices[0])
	}
	if rawJSONIDString(devices[0].AgentID) != "17" {
		t.Fatalf("agent id = %q, want 17", rawJSONIDString(devices[0].AgentID))
	}
}

func TestDecodeManagerDevicesTreatsNullDataAsEmpty(t *testing.T) {
	// 真机确认：manager 设备表为空时返回 {"data":null}，应当作"空列表"而非报错。
	devices, err := decodeManagerDevices([]byte(`{"data":null}`))
	if err != nil {
		t.Fatalf("decodeManagerDevices({data:null}): %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("devices = %+v, want empty", devices)
	}
}

func TestCheckManagerAccessAcceptsEmptyNullDeviceList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if err := c.CheckManagerAccess(context.Background()); err != nil {
		t.Fatalf("CheckManagerAccess with empty device list should pass, got: %v", err)
	}
}

func TestGetDeviceStatusReportsNotFoundOnEmptyNullDeviceList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	_, err := c.GetDeviceStatus(context.Background(), "dev-001")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found on empty device list", err)
	}
}

func TestDecodeManagerAgentParsesNestedAgentPayload(t *testing.T) {
	agent, err := decodeManagerAgent([]byte(`{
		"success": true,
		"data": {
			"agent": {
				"id": 9,
				"name": "care-agent",
				"custom_prompt": "old"
			}
		}
	}`))
	if err != nil {
		t.Fatalf("decodeManagerAgent: %v", err)
	}
	if agent["name"] != "care-agent" || agent["custom_prompt"] != "old" {
		t.Fatalf("agent = %v, want nested payload", agent)
	}
}

func TestGetHistoryReadsManagerHistoryEndpoint(t *testing.T) {
	firstAt := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	secondAt := time.Date(2026, 6, 1, 8, 30, 5, 0, time.UTC)
	var gotPath, gotToken, gotDeviceID, gotPageSize string
	var gotLegacyDeviceID, gotLegacyLimit string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		gotDeviceID = r.URL.Query().Get("device_id")
		gotPageSize = r.URL.Query().Get("page_size")
		gotLegacyDeviceID = r.URL.Query().Get("deviceId")
		gotLegacyLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"messages": [
					{
						"role": "user",
						"content": "我孙子叫啥",
						"created_at": "2026-06-01T08:30:00Z"
					},
					{
						"role": "assistant",
						"text": "小宝今天 7 岁啦",
						"at": "2026-06-01T08:30:05Z"
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	history, err := c.GetHistory(context.Background(), "dev-001", 2)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if gotPath != "/api/open/v1/history/messages" {
		t.Fatalf("path = %q, want history endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if gotDeviceID != "dev-001" || gotPageSize != "2" {
		t.Fatalf("query device_id=%q page_size=%q, want dev-001/2", gotDeviceID, gotPageSize)
	}
	if gotLegacyDeviceID != "" || gotLegacyLimit != "" {
		t.Fatalf("legacy query deviceId=%q limit=%q, want both empty", gotLegacyDeviceID, gotLegacyLimit)
	}
	if len(history) != 2 {
		t.Fatalf("history = %+v, want 2 messages", history)
	}
	if history[0].Role != "user" || history[0].Text != "我孙子叫啥" || !history[0].At.Equal(firstAt) {
		t.Fatalf("history[0] = %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Text != "小宝今天 7 岁啦" || !history[1].At.Equal(secondAt) {
		t.Fatalf("history[1] = %+v", history[1])
	}
}

func TestGetHistoryParsesDirectArrayPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"role":"user","message":"今天腰有点酸","timestamp":"2026-06-01T09:00:00Z"}
		]`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	history, err := c.GetHistory(context.Background(), "dev-001", 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 || history[0].Text != "今天腰有点酸" {
		t.Fatalf("history = %+v, want direct array message", history)
	}
}

func TestGetHistoryParsesNestedRecordsPayload(t *testing.T) {
	at := time.Date(2026, 6, 1, 9, 5, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"records": [
					{"role":"assistant","content":"您刚才说腰酸，要不要早点休息？","created_at":"2026-06-01T09:05:00Z"}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	history, err := c.GetHistory(context.Background(), "dev-001", 1)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history = %+v, want one nested records message", history)
	}
	if history[0].Role != "assistant" || history[0].Text != "您刚才说腰酸，要不要早点休息？" {
		t.Fatalf("history[0] = %+v, want assistant record", history[0])
	}
	if !history[0].At.Equal(at) {
		t.Fatalf("history[0].At = %s, want %s", history[0].At, at)
	}
}

func TestGetHistoryParsesUnixNumericTimestamp(t *testing.T) {
	at := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"role":"user","content":"今天腰有点酸","created_at":1780304400}
			]
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	history, err := c.GetHistory(context.Background(), "dev-001", 1)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 || !history[0].At.Equal(at) {
		t.Fatalf("history = %+v, want one message at %s", history, at)
	}
}

func TestGetHistoryRejectsInvalidTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"role":"user","content":"hi","created_at":"not-a-time"}]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if _, err := c.GetHistory(context.Background(), "dev-001", 5); err == nil {
		t.Fatal("expected invalid time error, got nil")
	}
}

func TestGetHistoryRejectsMalformedObjectWithoutList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"unexpected":true}}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if _, err := c.GetHistory(context.Background(), "dev-001", 5); err == nil {
		t.Fatal("expected malformed history response error, got nil")
	}
}

func TestCallDeviceMCPToolResolvesManagerDeviceIDAndSendsMCPContract(t *testing.T) {
	var gotPath, gotMethod, gotToken string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":1,"device_name":"9c:13:9e:8b:af:28","online":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices/1/mcp-tools":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"self.camera.take_photo"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/open/v1/devices/1/mcp-call":
			gotPath = r.URL.Path
			gotMethod = r.Method
			gotToken = r.Header.Get("X-API-Token")
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotBody)
			_, _ = w.Write([]byte(`{"success":true,"data":{"imageUrl":"https://example.test/capture.jpg"}}`))
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	raw, err := c.CallDeviceMCPTool(context.Background(), "9c:13:9e:8b:af:28", "self.camera.take_photo", map[string]any{"question": "请拍照看一眼"})
	if err != nil {
		t.Fatalf("CallDeviceMCPTool: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/open/v1/devices/1/mcp-call" {
		t.Fatalf("path = %q, want device mcp call endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if gotBody["tool_name"] != "self.camera.take_photo" {
		t.Fatalf("tool_name = %v, want self.camera.take_photo", gotBody["tool_name"])
	}
	if _, ok := gotBody["tool"]; ok {
		t.Fatalf("legacy tool field should not be sent: %v", gotBody)
	}
	arguments, ok := gotBody["arguments"].(map[string]any)
	if !ok || arguments["question"] != "请拍照看一眼" {
		t.Fatalf("arguments = %v, want question", gotBody["arguments"])
	}
	if _, ok := gotBody["args"]; ok {
		t.Fatalf("legacy args field should not be sent: %v", gotBody)
	}

	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal raw payload: %v", err)
	}
	if payload["imageUrl"] != "https://example.test/capture.jpg" {
		t.Fatalf("payload = %v, want imageUrl", payload)
	}
}

func TestCallDeviceMCPToolResolvesManagerSanitizedToolName(t *testing.T) {
	var gotToolName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"device_name":"dev-001"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices/1/mcp-tools":
			_, _ = w.Write([]byte(`{"data":{"tools":[{"name":"self_camera_take_photo"}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/open/v1/devices/1/mcp-call":
			var body mcpCallReq
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode mcp call: %v", err)
			}
			gotToolName = body.ToolName
			_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if _, err := c.CallDeviceMCPTool(context.Background(), "dev-001", "self.camera.take_photo", map[string]any{"question": "请看一眼"}); err != nil {
		t.Fatalf("CallDeviceMCPTool: %v", err)
	}
	if gotToolName != "self_camera_take_photo" {
		t.Fatalf("tool_name = %q, want manager-exposed sanitized name", gotToolName)
	}
}

func TestCallDeviceMCPToolReturnsDirectPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"data":[{"id":2,"device_id":"dev-001","online":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices/2/mcp-tools":
			_, _ = w.Write([]byte(`{"data":[{"name":"camera.capture"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/open/v1/devices/2/mcp-call":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	raw, err := c.CallDeviceMCPTool(context.Background(), "dev-001", "camera.capture", nil)
	if err != nil {
		t.Fatalf("CallDeviceMCPTool: %v", err)
	}
	var payload map[string]bool
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal direct payload: %v", err)
	}
	if !payload["ok"] {
		t.Fatalf("payload = %v, want ok true", payload)
	}
}

func TestCallDeviceMCPToolClassifiesOfflineDevice(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"device_name":"dev-offline","online":false}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/open/v1/devices/1/mcp-call":
			posted = true
			t.Fatalf("mcp-call should not be sent for an offline device")
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	_, err := c.CallDeviceMCPTool(context.Background(), "dev-offline", "self.camera.take_photo", nil)
	if !errors.Is(err, ErrDeviceOffline) {
		t.Fatalf("err = %v, want ErrDeviceOffline", err)
	}
	if posted {
		t.Fatal("mcp-call was sent for an offline device")
	}
}

func TestCallDeviceMCPToolClassifiesUnavailableTool(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"device_name":"dev-001","online":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices/1/mcp-tools":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/open/v1/devices/1/mcp-call":
			posted = true
			t.Fatalf("mcp-call should not be sent when the tool is unavailable")
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	_, err := c.CallDeviceMCPTool(context.Background(), "dev-001", "self.camera.take_photo", nil)
	if !errors.Is(err, ErrMCPToolUnavailable) {
		t.Fatalf("err = %v, want ErrMCPToolUnavailable", err)
	}
	if posted {
		t.Fatal("mcp-call was sent when the tool is unavailable")
	}
}

func TestCallDeviceMCPToolClassifiesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices":
			_, _ = w.Write([]byte(`{"data":[{"id":1,"device_name":"dev-001","online":true}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/open/v1/devices/1/mcp-tools":
			time.Sleep(50 * time.Millisecond)
			_, _ = w.Write([]byte(`{"data":[{"name":"self.camera.take_photo"}]}`))
		default:
			t.Fatalf("unexpected manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	c := NewHTTPClient(srv.URL, "tok_abc")
	_, err := c.CallDeviceMCPTool(ctx, "dev-001", "self.camera.take_photo", nil)
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("err = %v, want ErrUpstreamTimeout", err)
	}
}
