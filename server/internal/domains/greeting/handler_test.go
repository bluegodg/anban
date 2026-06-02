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

func TestHandlerGreetingScheduleGetAndPut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	putBody := `{"deviceId":"dev-001","slots":[{"label":"morning","time":"07:30","enabled":true,"tonePreset":"warm"},{"label":"noon","time":"12:20","enabled":false,"tonePreset":"casual"},{"label":"evening","time":"18:10","enabled":true}]}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/greetings/schedule", strings.NewReader(putBody))
	putW := httptest.NewRecorder()
	r.ServeHTTP(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body=%s", putW.Code, putW.Body.String())
	}
	var updated GreetingSchedule
	if err := json.Unmarshal(putW.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal updated schedule: %v", err)
	}
	if len(updated.Slots) != 3 || updated.Slots[0].Time != "07:30" {
		t.Fatalf("updated schedule = %+v, want three updated slots", updated)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/greetings/schedule?deviceId=dev-001", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", getW.Code, getW.Body.String())
	}
	var got GreetingSchedule
	if err := json.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal got schedule: %v", err)
	}
	if got.Slots[1].Enabled {
		t.Fatalf("got slots = %+v, want noon disabled", got.Slots)
	}
}

func TestHandlerGreetingScheduleRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "GET missing device", method: http.MethodGet, path: "/api/greetings/schedule"},
		{name: "PUT invalid JSON", method: http.MethodPut, path: "/api/greetings/schedule", body: `{not-json`},
		{name: "PUT missing device", method: http.MethodPut, path: "/api/greetings/schedule", body: `{"slots":[{"label":"morning","time":"08:00","enabled":true}]}`},
		{name: "PUT bad time", method: http.MethodPut, path: "/api/greetings/schedule", body: `{"deviceId":"dev-001","slots":[{"label":"morning","time":"8am","enabled":true}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
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
