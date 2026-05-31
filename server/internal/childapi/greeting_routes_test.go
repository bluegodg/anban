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
}

type greetingRoutesStub struct{}

func (greetingRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/greetings/trigger", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"greetingId": "stub-greeting"})
	})
}
