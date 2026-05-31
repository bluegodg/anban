package message

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

func TestHandlerCreateAndListMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"deviceId":"dev-001","text":"晚饭吃了吗","fromName":"小明"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var created Message
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created: %v", err)
	}
	if created.Status != StatusPlayed {
		t.Fatalf("created status = %q, want %q", created.Status, StatusPlayed)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/messages?deviceId=dev-001", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}

	var payload struct {
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].ID != created.ID {
		t.Fatalf("messages = %+v, want created message", payload.Messages)
	}
}

func TestHandlerCreateRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing fields", body: `{"deviceId":"","text":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerCreateReturnsBadGatewayWhenInjectFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, failingClient{err: errors.New("manager unavailable")})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"deviceId":"dev-001","text":"今晚记得吃饭"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"failed"`) {
		t.Fatalf("body = %s, want failed message payload", w.Body.String())
	}
}
