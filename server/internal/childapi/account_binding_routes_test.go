package childapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/domains/account"
	"github.com/bluegodg/anban/server/internal/domains/devicebinding"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/gin-gonic/gin"
)

func newAccountBindingRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	accountStore := account.NewStore(st.DB)
	if err := accountStore.AutoMigrate(); err != nil {
		t.Fatalf("account migrate: %v", err)
	}
	accountService := account.NewService(accountStore, account.Options{
		Now: func() time.Time { return time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC) },
	})
	bindingStore := devicebinding.NewStore(st.DB)
	if err := bindingStore.AutoMigrate(); err != nil {
		t.Fatalf("binding migrate: %v", err)
	}
	bindingService := devicebinding.NewService(bindingStore, devicebinding.Options{
		Now: func() time.Time { return time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC) },
		CodeGenerator: func() (string, error) {
			return "ANBAN-222222", nil
		},
	})
	if _, err := bindingService.EnsureDevice(t.Context(), devicebinding.DeviceSeed{
		DeviceID:         "dev-001",
		BindingCode:      "ANBAN-111111",
		DisplayName:      "客厅安伴",
		ElderDisplayName: "王阿姨",
	}); err != nil {
		t.Fatalf("EnsureDevice error = %v", err)
	}

	return NewRouter(Deps{
		AccessCode:           "demo",
		AccountService:       accountService,
		DeviceBindingService: bindingService,
		MessageRoutes:        messageContextEchoRoutes{},
		ProfileRoutes:        profileContextEchoRoutes{},
	})
}

func TestAuthMeAndBindDeviceFlow(t *testing.T) {
	r := newAccountBindingRouter(t)

	register := httptest.NewRecorder()
	r.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/auth/register", `{"phone":"13800000000","password":"secret123","nickname":"小兰"}`))
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d; body=%s", register.Code, register.Body.String())
	}
	token := extractJSONField(t, register.Body.String(), "token")
	if token == "" {
		t.Fatalf("register response missing token: %s", register.Body.String())
	}

	me := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(me, req)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"binding":null`) {
		t.Fatalf("unbound me status=%d body=%s", me.Code, me.Body.String())
	}

	bind := httptest.NewRecorder()
	req = jsonRequest(http.MethodPost, "/api/device-binding", `{"role":"admin","bindingCode":"ANBAN-111111"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(bind, req)
	if bind.Code != http.StatusCreated || !strings.Contains(bind.Body.String(), `"role":"admin"`) {
		t.Fatalf("bind status=%d body=%s", bind.Code, bind.Body.String())
	}

	me = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(me, req)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"deviceId":"dev-001"`) {
		t.Fatalf("bound me status=%d body=%s", me.Code, me.Body.String())
	}
}

func TestUnboundBearerCannotUseDeviceAPI(t *testing.T) {
	r := newAccountBindingRouter(t)
	register := httptest.NewRecorder()
	r.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/auth/register", `{"phone":"13800000001","password":"secret123"}`))
	token := extractJSONField(t, register.Body.String(), "token")

	w := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/api/messages", `{"text":"hi"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "device_not_bound") {
		t.Fatalf("unbound device api status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBearerDeviceAPIReceivesBoundDeviceAndSender(t *testing.T) {
	r := newAccountBindingRouter(t)
	register := httptest.NewRecorder()
	r.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/auth/register", `{"phone":"13800000002","password":"secret123","nickname":"小兰"}`))
	token := extractJSONField(t, register.Body.String(), "token")
	req := jsonRequest(http.MethodPost, "/api/device-binding", `{"role":"member","bindingCode":"ANBAN-111111"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), req)

	w := httptest.NewRecorder()
	req = jsonRequest(http.MethodPost, "/api/messages", `{"deviceId":"evil-device","fromName":"别人","text":"hi"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("message status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"deviceId":"dev-001"`) || !strings.Contains(w.Body.String(), `"sender":"小兰"`) {
		t.Fatalf("message context body=%s", w.Body.String())
	}
}

func TestMemberCannotWriteProfileButAccessCodeStillCan(t *testing.T) {
	r := newAccountBindingRouter(t)
	register := httptest.NewRecorder()
	r.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/auth/register", `{"phone":"13800000003","password":"secret123","nickname":"成员"}`))
	token := extractJSONField(t, register.Body.String(), "token")
	req := jsonRequest(http.MethodPost, "/api/device-binding", `{"role":"member","bindingCode":"ANBAN-111111"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), req)

	w := httptest.NewRecorder()
	req = jsonRequest(http.MethodPut, "/api/profile", `{"fields":{"name":"妈妈"}}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "admin_required") {
		t.Fatalf("member profile status=%d body=%s", w.Code, w.Body.String())
	}

	compat := httptest.NewRecorder()
	req = jsonRequest(http.MethodPut, "/api/profile", `{"deviceId":"dev-001","fields":{"name":"妈妈"}}`)
	req.Header.Set("X-Access-Code", "demo")
	r.ServeHTTP(compat, req)
	if compat.Code != http.StatusOK {
		t.Fatalf("access code profile status=%d body=%s", compat.Code, compat.Body.String())
	}
}

func TestMemberAuthenticationAllowsCareRoutes(t *testing.T) {
	r := newAccountBindingRouter(t)
	token := registerAndBind(t, r, "13800000006", "member-care", "member")

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/device/status"},
		{method: http.MethodGet, path: "/api/device/history"},
		{method: http.MethodGet, path: "/api/reminders"},
		{method: http.MethodPost, path: "/api/reminders", body: `{}`},
		{method: http.MethodPost, path: "/api/greetings/trigger", body: `{}`},
		{method: http.MethodPost, path: "/api/vision/capture", body: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer "+token)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			r.ServeHTTP(w, req)
			if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden || w.Code == http.StatusConflict {
				t.Fatalf("member care route status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminCanManageMembersAndResetCode(t *testing.T) {
	r := newAccountBindingRouter(t)
	adminToken := registerAndBind(t, r, "13800000004", "admin", "admin")
	memberToken := registerAndBind(t, r, "13800000005", "member", "member")
	_ = memberToken

	members := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/device-binding/members", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(members, req)
	if members.Code != http.StatusOK || !strings.Contains(members.Body.String(), `"nickname":"member"`) {
		t.Fatalf("members status=%d body=%s", members.Code, members.Body.String())
	}

	reset := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/device-binding/reset-code", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(reset, req)
	if reset.Code != http.StatusOK || !strings.Contains(reset.Body.String(), "ANBAN-222222") {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}
}

func registerAndBind(t *testing.T, r *gin.Engine, phone, nickname, role string) string {
	t.Helper()
	register := httptest.NewRecorder()
	r.ServeHTTP(register, jsonRequest(http.MethodPost, "/api/auth/register", `{"phone":"`+phone+`","password":"secret123","nickname":"`+nickname+`"}`))
	token := extractJSONField(t, register.Body.String(), "token")
	req := jsonRequest(http.MethodPost, "/api/device-binding", `{"role":"`+role+`","bindingCode":"ANBAN-111111"}`)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), req)
	return token
}

func jsonRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func extractJSONField(t *testing.T, body, field string) string {
	t.Helper()
	needle := `"` + field + `":"`
	start := strings.Index(body, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}

type messageContextEchoRoutes struct{}

func (messageContextEchoRoutes) RegisterRoutes(r gin.IRoutes) {
	r.POST("/messages", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{
			"deviceId": c.GetString("anban.deviceID"),
			"sender":   c.GetString("anban.senderDisplayName"),
		})
	})
	r.GET("/messages", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": c.GetString("anban.deviceID")})
	})
}

type profileContextEchoRoutes struct{}

func (profileContextEchoRoutes) RegisterRoutes(r gin.IRoutes) {
	r.GET("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": c.GetString("anban.deviceID")})
	})
	r.PUT("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": c.GetString("anban.deviceID")})
	})
	r.POST("/profile", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"deviceId": c.GetString("anban.deviceID")})
	})
}
