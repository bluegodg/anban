package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	"github.com/gin-gonic/gin"
)

func TestHandlerGetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lastActive := time.Date(2026, 6, 1, 8, 30, 0, 0, time.UTC)
	svc := NewService(&statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
	})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/status?deviceId=dev-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/status status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var snapshot Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if snapshot.DeviceID != "dev-001" || !snapshot.Online {
		t.Fatalf("snapshot = %+v, want online dev-001", snapshot)
	}
	if snapshot.LastSeenAt == nil || !snapshot.LastSeenAt.Equal(lastActive) {
		t.Fatalf("lastSeenAt = %v, want %s", snapshot.LastSeenAt, lastActive)
	}
}

func TestHandlerSupportsPRDDeviceStatusRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&statusClient{
		status: xiaozhiclient.DeviceStatus{
			DeviceID: "dev-001",
			Online:   true,
		},
	})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/device/status?deviceId=dev-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/device/status status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerGetHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	at := time.Date(2026, 6, 1, 8, 31, 0, 0, time.UTC)
	xc := &statusClient{
		history: []xiaozhiclient.HistoryMessage{
			{Role: "user", Text: "今天腰有点酸", At: at},
		},
	}
	svc := NewService(xc)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/device/history?deviceId=dev-001&limit=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/device/history status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if xc.gotHistoryDeviceID != "dev-001" || xc.gotHistoryLimit != 1 {
		t.Fatalf("history call = (%q, %d), want (dev-001, 1)", xc.gotHistoryDeviceID, xc.gotHistoryLimit)
	}

	var history HistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
		t.Fatalf("unmarshal history: %v", err)
	}
	if history.DeviceID != "dev-001" || len(history.Messages) != 1 {
		t.Fatalf("history = %+v, want one message for dev-001", history)
	}
	if history.Messages[0].Role != "user" || history.Messages[0].Text != "今天腰有点酸" {
		t.Fatalf("history message = %+v, want user text", history.Messages[0])
	}
}

func TestHandlerGetStatusRejectsMissingDeviceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&statusClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/status status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "deviceId") {
		t.Fatalf("body = %s, want deviceId validation message", w.Body.String())
	}
}

func TestHandlerGetHistoryRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&statusClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/device/history?deviceId=dev-001&limit=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/device/history status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "limit") {
		t.Fatalf("body = %s, want limit validation message", w.Body.String())
	}
}
