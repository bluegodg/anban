package greeting

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	"github.com/gin-gonic/gin"
)

func TestHandlerTriggerGreeting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/greetings/trigger", strings.NewReader(`{"deviceId":"dev-001","tonePreset":"warm"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var payload Greeting
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Status != StatusPlayed {
		t.Fatalf("Status = %q, want %q", payload.Status, StatusPlayed)
	}
}

func TestHandlerTriggerGreetingRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing device", body: `{"deviceId":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/greetings/trigger", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerTriggerGreetingReturnsBadGatewayWhenInjectFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, failingClient{err: errors.New("manager unavailable")})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/greetings/trigger", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"failed"`) {
		t.Fatalf("body = %s, want failed greeting payload", w.Body.String())
	}
}
