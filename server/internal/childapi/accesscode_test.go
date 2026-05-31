package childapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAccessCodeMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireAccessCode("secret"))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 缺码 → 401
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("no code: status = %d, want 401", w1.Code)
	}

	// 正确码 → 200
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Access-Code", "secret")
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("with code: status = %d, want 200", w2.Code)
	}
}
