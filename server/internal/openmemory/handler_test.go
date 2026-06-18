package openmemory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/gin-gonic/gin"
)

type fakeProfileReader struct {
	profile profile.Profile
	err     error
}

func (f fakeProfileReader) Get(context.Context, string) (profile.Profile, error) {
	return f.profile, f.err
}

func TestSearchReturnsCurrentCompanionContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler("provider-secret", fakeProfileReader{profile: profile.Profile{
		DeviceID: "dev-001",
		Fields: profile.Fields{
			Name:          "蓝",
			Grandchildren: []string{"小宝"},
			Hobbies:       []string{"养花"},
		},
		MemoryFacts: []string{"老人喜欢饭后晒太阳"},
		MindContext: "最近较挂念老人，语气更关切些",
	}})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/openmem/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/openmem/v1/search/memory", strings.NewReader(`{
		"user_id":"dev-001",
		"conversation_id":"dev-001",
		"query":"我喜欢做什么",
		"memory_limit_number":10
	}`))
	req.Header.Set("Authorization", "Bearer provider-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data struct {
			Items []struct {
				Value string `json:"memory_value"`
			} `json:"memory_detail_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Data.Items) != 1 {
		t.Fatalf("items = %+v, want one assembled context", got.Data.Items)
	}
	for _, want := range []string{"陪伴对象姓名：蓝", "孙辈：小宝", "喜好：养花", "专属记忆：老人喜欢饭后晒太阳", "心智上下文：最近较挂念老人"} {
		if !strings.Contains(got.Data.Items[0].Value, want) {
			t.Fatalf("context = %q, want contains %q", got.Data.Items[0].Value, want)
		}
	}
}

func TestSearchReturnsNoStaticContextForEmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler("provider-secret", fakeProfileReader{profile: profile.Profile{
		DeviceID: "dev-001",
		Fields:   profile.Fields{Name: "蓝"},
	}})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/openmem/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/openmem/v1/search/memory", strings.NewReader(`{"user_id":"dev-001","query":""}`))
	req.Header.Set("Authorization", "Bearer provider-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"memory_detail_list":[]`) {
		t.Fatalf("status/body = %d %s, want empty dynamic search result", w.Code, w.Body.String())
	}
}

func TestProviderRejectsMissingOrWrongToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler("provider-secret", fakeProfileReader{})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/openmem/v1"))

	for _, auth := range []string{"", "Bearer wrong-secret"} {
		req := httptest.NewRequest(http.MethodPost, "/api/openmem/v1/search/memory", strings.NewReader(`{"user_id":"dev-001","query":"你好"}`))
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("auth %q status = %d, want 401", auth, w.Code)
		}
	}
}

func TestWriteProtocolIsAcknowledgedWithoutIngestingConversation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reader := fakeProfileReader{}
	h := NewHandler("provider-secret", reader)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/openmem/v1"))

	for _, path := range []string{"/add/message", "/flush", "/reset/memory"} {
		req := httptest.NewRequest(http.MethodPost, "/api/openmem/v1"+path, strings.NewReader(`{"user_id":"dev-001"}`))
		req.Header.Set("Authorization", "Bearer provider-secret")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ingest":"history_poller"`) {
			t.Fatalf("%s status/body = %d %s, want read-only acknowledgement", path, w.Code, w.Body.String())
		}
	}
}
