package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStatusRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:   "demo",
		StatusRoutes: statusRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status?deviceId=dev-001", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/status status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-status") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}
}

func TestStatusRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodGet, "/api/status?deviceId=dev-001", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/status status = %d, want 501", w.Code)
	}
}

type statusRoutesStub struct{}

func (statusRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-status"})
	})
}
