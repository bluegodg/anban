package xiaozhiclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestGetDeviceStatusReadsManagerDeviceEndpoint(t *testing.T) {
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	var gotPath, gotToken string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"device_id": "dev-001",
				"online": true,
				"last_active_at": "2026-06-01T08:30:00Z"
			}
		}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	status, err := c.GetDeviceStatus(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	if gotPath != "/api/open/v1/devices/dev-001" {
		t.Fatalf("path = %q, want device endpoint", gotPath)
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

func TestGetDeviceStatusParsesDirectActivePayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "dev-002",
			"status": "active",
			"last_seen_at": "2026-06-01T08:31:00Z"
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
		_, _ = w.Write([]byte(`{"device_id":"dev-001","last_active_at":"not-a-time"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	if _, err := c.GetDeviceStatus(context.Background(), "dev-001"); err == nil {
		t.Fatal("expected invalid time error, got nil")
	}
}

func TestSetRolePromptSendsManagerRequest(t *testing.T) {
	var gotPath, gotMethod, gotToken string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotToken = r.Header.Get("X-API-Token")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok_abc")
	err := c.SetRolePrompt(context.Background(), "dev-001", "请记住王阿姨喜欢豫剧")
	if err != nil {
		t.Fatalf("SetRolePrompt: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %q, want PUT", gotMethod)
	}
	if gotPath != "/api/open/v1/devices/dev-001/role-prompt" {
		t.Fatalf("path = %q, want role prompt endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q", gotToken)
	}
	if gotBody["prompt"] != "请记住王阿姨喜欢豫剧" {
		t.Fatalf("body = %v, want prompt", gotBody)
	}
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

func TestGetHistoryReadsManagerHistoryEndpoint(t *testing.T) {
	firstAt := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	secondAt := time.Date(2026, 6, 1, 8, 30, 5, 0, time.UTC)
	var gotPath, gotToken, gotDeviceID, gotLimit string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		gotDeviceID = r.URL.Query().Get("deviceId")
		gotLimit = r.URL.Query().Get("limit")
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
	if gotDeviceID != "dev-001" || gotLimit != "2" {
		t.Fatalf("query deviceId=%q limit=%q, want dev-001/2", gotDeviceID, gotLimit)
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

func TestUnimplementedMethodsReturnErrNotImplemented(t *testing.T) {
	c := NewHTTPClient("http://manager.local", "tok_abc")

	if _, err := c.CallDeviceMCPTool(context.Background(), "dev-001", "camera.capture", nil); err == nil {
		t.Fatal("CallDeviceMCPTool err = nil, want not implemented")
	}
}
