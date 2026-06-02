package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVisionRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:   "demo",
		VisionRoutes: visionRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/capture status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-capture") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}
}

func TestVisionRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{"deviceId":"dev-001"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/vision/capture status = %d, want 501", w.Code)
	}
}

type visionRoutesStub struct{}

func (visionRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/vision/capture", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"captureId": "stub-capture"})
	})
}
