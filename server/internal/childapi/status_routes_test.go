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

	prdReq := httptest.NewRequest(http.MethodGet, "/api/device/status?deviceId=dev-001", nil)
	prdReq.Header.Set("X-Access-Code", "demo")
	prdW := httptest.NewRecorder()
	r.ServeHTTP(prdW, prdReq)
	if prdW.Code != http.StatusOK {
		t.Fatalf("GET /api/device/status status = %d, want 200; body=%s", prdW.Code, prdW.Body.String())
	}
	if !strings.Contains(prdW.Body.String(), "stub-status") {
		t.Fatalf("body = %s, want stub response for PRD status path", prdW.Body.String())
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/device/history?deviceId=dev-001", nil)
	historyReq.Header.Set("X-Access-Code", "demo")
	historyW := httptest.NewRecorder()
	r.ServeHTTP(historyW, historyReq)
	if historyW.Code != http.StatusOK {
		t.Fatalf("GET /api/device/history status = %d, want 200; body=%s", historyW.Code, historyW.Body.String())
	}
	if !strings.Contains(historyW.Body.String(), "stub-history") {
		t.Fatalf("body = %s, want stub response for history path", historyW.Body.String())
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

	prdReq := httptest.NewRequest(http.MethodGet, "/api/device/status?deviceId=dev-001", nil)
	prdReq.Header.Set("X-Access-Code", "demo")
	prdW := httptest.NewRecorder()
	r.ServeHTTP(prdW, prdReq)
	if prdW.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/device/status status = %d, want 501", prdW.Code)
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/device/history?deviceId=dev-001", nil)
	historyReq.Header.Set("X-Access-Code", "demo")
	historyW := httptest.NewRecorder()
	r.ServeHTTP(historyW, historyReq)
	if historyW.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/device/history status = %d, want 501", historyW.Code)
	}
}

func TestStatusRoutesDisableCachingForFreshChildStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:   "demo",
		StatusRoutes: statusRoutesStub{},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/device/status?deviceId=dev-001", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/device/status status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store for fresh multi-browser status", got)
	}
	if got := w.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", got)
	}
}

type statusRoutesStub struct{}

func (statusRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-status"})
	})
	r.GET("/device/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-status"})
	})
	r.GET("/device/history", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-history"})
	})
}
