package profile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerUpdateAndGetProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &profileClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	body := `{"deviceId":"dev-001","fields":{"name":"王秀英","nickname":"妈","children":["小明"],"grandchildren":["小宝"],"hobbies":["豫剧"],"schedule":"早睡早起","health":"高血压","taboos":["甜食"]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/profile status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/profile?deviceId=dev-001", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GET /api/profile status = %d, want 200; body=%s", getW.Code, getW.Body.String())
	}
	var got Profile
	if err := json.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal profile: %v", err)
	}
	if got.DeviceID != "dev-001" || got.Fields.Nickname != "妈" {
		t.Fatalf("profile = %+v, want saved profile", got)
	}
}

func TestHandlerUpdateRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t, &profileClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPut, "/api/profile", strings.NewReader(`{"deviceId":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("PUT /api/profile status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}
