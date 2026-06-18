package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMemoryRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:   "demo",
		MemoryRoutes: memoryRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/memory/facts", strings.NewReader(`{"deviceId":"dev-001","text":"蓝喜欢养花"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/memory/facts status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-memory") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}
}

func TestMemoryRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodGet, "/api/memory/facts?deviceId=dev-001", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/memory/facts status = %d, want 501", w.Code)
	}
}

func TestMemoryWritePathsRequireAdminForAccountMode(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodGet, path: "/api/memory/facts", want: false},
		{method: http.MethodPost, path: "/api/memory/facts", want: true},
		{method: http.MethodPut, path: "/api/memory/facts/1", want: true},
		{method: http.MethodDelete, path: "/api/memory/facts/1", want: true},
	} {
		if got := isAdminOnlyWritePath(tc.method, tc.path); got != tc.want {
			t.Fatalf("isAdminOnlyWritePath(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

type memoryRoutesStub struct{}

func (memoryRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.GET("/memory/facts", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-memory"})
	})
	r.POST("/memory/facts", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-memory"})
	})
	r.PUT("/memory/facts/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-memory"})
	})
	r.DELETE("/memory/facts/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": "stub-memory"})
	})
}
