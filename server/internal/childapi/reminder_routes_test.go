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
}

type reminderRoutesStub struct{}

func (reminderRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/reminders", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"reminderId": "stub-reminder"})
	})
	r.GET("/reminders", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reminders": []string{"stub-reminder"}})
	})
}
