package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProfileRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:    "demo",
		ProfileRoutes: profileRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/profile status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-profile") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}
}

func TestProfileRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("PUT /api/profile status = %d, want 501", w.Code)
	}
}

type profileRoutesStub struct{}

func (profileRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.GET("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-profile"})
	})
	r.PUT("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-profile"})
	})
	r.POST("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-profile"})
	})
}
