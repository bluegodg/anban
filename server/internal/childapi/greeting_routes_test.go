package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGreetingRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:     "demo",
		GreetingRoutes: greetingRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/greetings/trigger", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/greetings/trigger status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-greeting") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}

	scheduleReq := httptest.NewRequest(http.MethodGet, "/api/greetings/schedule?deviceId=dev-001", nil)
	scheduleReq.Header.Set("X-Access-Code", "demo")
	scheduleW := httptest.NewRecorder()
	r.ServeHTTP(scheduleW, scheduleReq)
	if scheduleW.Code != http.StatusOK {
		t.Fatalf("GET /api/greetings/schedule status = %d, want 200; body=%s", scheduleW.Code, scheduleW.Body.String())
	}
	if !strings.Contains(scheduleW.Body.String(), "stub-schedule") {
		t.Fatalf("body = %s, want stub schedule response", scheduleW.Body.String())
	}
}

func TestGreetingRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodPost, "/api/greetings/trigger", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/greetings/trigger status = %d, want 501", w.Code)
	}

	scheduleReq := httptest.NewRequest(http.MethodGet, "/api/greetings/schedule?deviceId=dev-001", nil)
	scheduleReq.Header.Set("X-Access-Code", "demo")
	scheduleW := httptest.NewRecorder()
	r.ServeHTTP(scheduleW, scheduleReq)
	if scheduleW.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/greetings/schedule status = %d, want 501", scheduleW.Code)
	}
}

type greetingRoutesStub struct{}

func (greetingRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/greetings/trigger", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"greetingId": "stub-greeting"})
	})
	r.GET("/greetings/schedule", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"scheduleId": "stub-schedule"})
	})
	r.PUT("/greetings/schedule", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"scheduleId": "stub-schedule"})
	})
}
