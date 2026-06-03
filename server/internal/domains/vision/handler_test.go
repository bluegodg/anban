package vision

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`)})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{
		"deviceId":"dev-001",
		"tool":"camera.capture",
		"args":{"quality":"low"}
	}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/capture status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var payload CaptureResult
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal capture: %v", err)
	}
	if payload.DeviceID != "dev-001" || payload.Tool != "camera.capture" {
		t.Fatalf("payload = %+v, want dev-001 camera.capture", payload)
	}
	if string(payload.Raw) != `{"imageUrl":"https://example.test/capture.jpg"}` {
		t.Fatalf("raw = %s", payload.Raw)
	}
}

func TestHandlerCaptureRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{})
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
			req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerCaptureReturnsBadGatewayWhenMCPFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{err: errors.New("manager unavailable")})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerCheckPresenceCapturesAndObserves(t *testing.T) {
	gin.SetMode(gin.TestMode)
	xc := &visionClient{raw: json.RawMessage(`{"presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{}
	svc := NewService(xc, trigger)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST check-presence no_one status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	xc.raw = json.RawMessage(`{"presence":"someone"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST check-presence someone status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one return greeting", trigger.calls)
	}

	var payload PresenceCheckResult
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal check presence: %v", err)
	}
	if !payload.Observation.TriggeredGreeting {
		t.Fatalf("payload = %+v, want triggered greeting", payload)
	}
}

func TestHandlerCheckPresenceReturnsBadGatewayWhenPresenceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`)})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerObservePresenceTriggersGreeting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	trigger := &fakeGreetingTrigger{}
	svc := NewService(&visionClient{}, trigger)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	for _, body := range []string{
		`{"deviceId":"dev-001","presence":"no_one"}`,
		`{"deviceId":"dev-001","presence":"someone"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("POST /api/vision/presence status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one no_one -> someone greeting", trigger.calls)
	}
}

func TestHandlerObservePresenceRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{}, &fakeGreetingTrigger{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing device", body: `{"deviceId":"","presence":"someone"}`},
		{name: "bad presence", body: `{"deviceId":"dev-001","presence":"maybe"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}
