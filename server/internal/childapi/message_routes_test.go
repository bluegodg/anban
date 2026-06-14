package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMessageRoutesAreRegisteredWhenDependencyProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{
		AccessCode:    "demo",
		MessageRoutes: messageRoutesStub{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"deviceId":"dev-001","text":"hi"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/messages status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "stub-message") {
		t.Fatalf("body = %s, want stub response", w.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/messages?deviceId=dev-001", nil)
	listReq.Header.Set("X-Access-Code", "demo")
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("GET /api/messages status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}
	if !strings.Contains(listW.Body.String(), "stub-message") {
		t.Fatalf("body = %s, want stub list response", listW.Body.String())
	}
}

func TestMessageRoutesStayPlaceholderWhenDependencyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(Deps{AccessCode: "demo"})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", strings.NewReader(`{"deviceId":"dev-001","text":"hi"}`))
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("POST /api/messages status = %d, want 501", w.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/messages?deviceId=dev-001", nil)
	listReq.Header.Set("X-Access-Code", "demo")
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusNotImplemented {
		t.Fatalf("GET /api/messages status = %d, want 501", listW.Code)
	}
}

type messageRoutesStub struct{}

func (messageRoutesStub) RegisterRoutes(r gin.IRoutes) {
	r.POST("/messages", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"messageId": "stub-message"})
	})
	r.GET("/messages", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"messages": []string{"stub-message"}})
	})
}
