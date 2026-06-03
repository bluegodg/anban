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

	presenceReq := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(`{"deviceId":"dev-001","presence":"someone"}`))
	presenceReq.Header.Set("X-Access-Code", "demo")
	presenceW := httptest.NewRecorder()
	r.ServeHTTP(presenceW, presenceReq)

	if presenceW.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/presence status = %d, want 200; body=%s", presenceW.Code, presenceW.Body.String())
	}
	if !strings.Contains(presenceW.Body.String(), "stub-presence") {
		t.Fatalf("body = %s, want stub response", presenceW.Body.String())
	}

	checkReq := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	checkReq.Header.Set("X-Access-Code", "demo")
	checkW := httptest.NewRecorder()
	r.ServeHTTP(checkW, checkReq)

	if checkW.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/check-presence status = %d, want 200; body=%s", checkW.Code, checkW.Body.String())
	}
	if !strings.Contains(checkW.Body.String(), "stub-check-presence") {
		t.Fatalf("body = %s, want stub response", checkW.Body.String())
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

	presenceReq := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(`{"deviceId":"dev-001","presence":"someone"}`))
	presenceReq.Header.Set("X-Access-Code", "demo")
	presenceW := httptest.NewRecorder()
	r.ServeHTTP(presenceW, presenceReq)

	if presenceW.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/vision/presence status = %d, want 501", presenceW.Code)
	}

	checkReq := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	checkReq.Header.Set("X-Access-Code", "demo")
	checkW := httptest.NewRecorder()
	r.ServeHTTP(checkW, checkReq)

	if checkW.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/vision/check-presence status = %d, want 501", checkW.Code)
	}
}

type visionRoutesStub struct{}

func (visionRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/vision/capture", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"captureId": "stub-capture"})
	})
	r.POST("/vision/presence", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"presenceId": "stub-presence"})
	})
	r.POST("/vision/check-presence", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"checkId": "stub-check-presence"})
	})
}
