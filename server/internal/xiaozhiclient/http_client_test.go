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
