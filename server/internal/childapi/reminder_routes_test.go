package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReminderRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:     "demo",
		ReminderRoutes: reminderRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reminders", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/reminders status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-reminder") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}

	ackReq := httptest.NewRequest(http.MethodPost, "/api/reminders/1/ack", strings.NewReader(`{"ackKind":"voice"}`))
	ackReq.Header.Set("X-Access-Code", "demo")
	ackW := httptest.NewRecorder()
	r.ServeHTTP(ackW, ackReq)
	if ackW.Code != http.StatusOK {
		t.Fatalf("POST /api/reminders/:id/ack status = %d, want 200; body=%s", ackW.Code, ackW.Body.String())
	}
}

func TestReminderRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodPost, "/api/reminders", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/reminders status = %d, want 501", w.Code)
	}

	ackReq := httptest.NewRequest(http.MethodPost, "/api/reminders/1/ack", strings.NewReader(`{"ackKind":"voice"}`))
	ackReq.Header.Set("X-Access-Code", "demo")
	ackW := httptest.NewRecorder()
	r.ServeHTTP(ackW, ackReq)
	if ackW.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/reminders/:id/ack status = %d, want 501", ackW.Code)
	}
}

type reminderRoutesStub struct{}

func (reminderRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/reminders", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"reminderId": "stub-reminder"})
	})
	r.GET("/reminders", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reminders": []string{"stub-reminder"}})
	})
	r.POST("/reminders/:id/ack", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reminderId": c.Param("id"), "status": "completed"})
	})
}
