package reminder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	"github.com/gin-gonic/gin"
)

func TestHandlerCreateAndListReminders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	body := `{"deviceId":"dev-001","scheduledAt":"2026-06-01T09:01:30Z","content":"测血压","category":"med","recurrence":"daily","important":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/reminders", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var created Reminder
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created: %v", err)
	}
	if created.Status != StatusScheduled {
		t.Fatalf("Status = %q, want %q", created.Status, StatusScheduled)
	}
	if created.Recurrence != RecurrenceDaily || !created.Important {
		t.Fatalf("recurrence/important = %q/%v, want daily/true", created.Recurrence, created.Important)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/reminders?deviceId=dev-001", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}
	var payload struct {
		Reminders []Reminder `json:"reminders"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(payload.Reminders) != 1 || payload.Reminders[0].ID != created.ID {
		t.Fatalf("reminders = %+v, want created reminder", payload.Reminders)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/reminders/1", nil)
	deleteW := httptest.NewRecorder()
	r.ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200; body=%s", deleteW.Code, deleteW.Body.String())
	}
	var canceled Reminder
	if err := json.Unmarshal(deleteW.Body.Bytes(), &canceled); err != nil {
		t.Fatalf("unmarshal canceled: %v", err)
	}
	if canceled.Status != StatusCanceled {
		t.Fatalf("canceled status = %q, want %q", canceled.Status, StatusCanceled)
	}
}

func TestHandlerCreateRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing fields", body: `{"deviceId":"","content":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/reminders", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerAcknowledgeReminder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	created, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: svc.now().Add(time.Minute),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)

	body := `{"ackKind":"voice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/reminders/1/ack", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST ack status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var completed Reminder
	if err := json.Unmarshal(w.Body.Bytes(), &completed); err != nil {
		t.Fatalf("unmarshal completed: %v", err)
	}
	if completed.ID != created.ID || completed.Status != StatusCompleted || completed.AckKind != AckKindVoice {
		t.Fatalf("completed = %+v, want completed voice ack for reminder %d", completed, created.ID)
	}
}

func TestHandlerCancelReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerCancelRejectsBadID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodDelete, "/api/reminders/not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("DELETE status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerAcknowledgeRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	scheduled, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 10, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "bad id", path: "/api/reminders/not-a-number/ack", body: `{}`, status: http.StatusBadRequest},
		{name: "invalid JSON", path: "/api/reminders/1/ack", body: `{not-json`, status: http.StatusBadRequest},
		{name: "not found", path: "/api/reminders/99/ack", body: `{}`, status: http.StatusNotFound},
		{name: "not yet played", path: "/api/reminders/" + strconv.Itoa(int(scheduled.ID)) + "/ack", body: `{}`, status: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("POST ack status = %d, want %d; body=%s", w.Code, tt.status, w.Body.String())
			}
		})
	}
}
