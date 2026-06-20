package mindview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/childapi"
	"github.com/bluegodg/anban/server/internal/domains/account"
	"github.com/bluegodg/anban/server/internal/domains/devicebinding"
	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/gin-gonic/gin"
)

func TestMindRoutesExposeSnapshotWithLegacyAccessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ms, _, _, _ := newMindRouteFixture(t)
	saveRouteSnapshotState(t, ms, "dev-001", "轻轻留意老人状态", 0.81)

	r := childapi.NewRouter(childapi.Deps{
		AccessCode: "demo",
		MindRoutes: NewHandler(mind.NewReadService(ms)),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/mind/snapshot?deviceId=dev-001", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"\"available\":true", "\"todayTheme\":\"轻轻留意老人状态\"", "\"warmth\":0.81"} {
		if !strings.Contains(body, want) {
			t.Fatalf("snapshot body = %s, want %s", body, want)
		}
	}
	if strings.Contains(body, "familyWeight") || strings.Contains(body, "processedEvent") {
		t.Fatalf("snapshot leaked internal state fields: %s", body)
	}
}

func TestMindRoutesUseBoundDeviceForAccountMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ms, accountService, bindingService, _ := newMindRouteFixture(t)
	saveRouteSnapshotState(t, ms, "dev-001", "绑定设备的心智", 0.66)
	saveRouteSnapshotState(t, ms, "evil-device", "错误设备的心智", 0.12)

	r := childapi.NewRouter(childapi.Deps{
		AccessCode:           "demo",
		AccountService:       accountService,
		DeviceBindingService: bindingService,
		MindRoutes:           NewHandler(mind.NewReadService(ms)),
	})
	token := registerAndBindMindRouteAccount(t, r, "13800001000", "member")

	req := httptest.NewRequest(http.MethodGet, "/api/mind/snapshot?deviceId=evil-device", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "绑定设备的心智") || strings.Contains(body, "错误设备的心智") {
		t.Fatalf("account snapshot body=%s, want bound device only", body)
	}
}

func TestMindRoutesRejectInvalidTimelineCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ms, _, _, _ := newMindRouteFixture(t)
	r := childapi.NewRouter(childapi.Deps{
		AccessCode: "demo",
		MindRoutes: NewHandler(mind.NewReadService(ms)),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/mind/timeline?deviceId=dev-001&cursor=not-valid", nil)
	req.Header.Set("X-Access-Code", "demo")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_cursor") {
		t.Fatalf("timeline invalid cursor status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMindRoutesRejectInvalidTimelineKindAndLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ms, _, _, _ := newMindRouteFixture(t)
	r := childapi.NewRouter(childapi.Deps{
		AccessCode: "demo",
		MindRoutes: NewHandler(mind.NewReadService(ms)),
	})
	tests := []struct {
		path string
		code string
	}{
		{path: "/api/mind/timeline?deviceId=dev-001&kind=raw-table", code: "invalid_kind"},
		{path: "/api/mind/timeline?deviceId=dev-001&limit=0", code: "invalid_limit"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("X-Access-Code", "demo")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), tt.code) {
				t.Fatalf("%s status=%d body=%s", tt.path, w.Code, w.Body.String())
			}
		})
	}
}

func newMindRouteFixture(t *testing.T) (*mind.Store, *account.Service, *devicebinding.Service, func()) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	sqlDB, err := st.DB.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close db: %v", err)
		}
	}
	t.Cleanup(cleanup)

	accountStore := account.NewStore(st.DB)
	if err := accountStore.AutoMigrate(); err != nil {
		t.Fatalf("account migrate: %v", err)
	}
	accountService := account.NewService(accountStore, account.Options{
		Now: func() time.Time { return time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC) },
	})
	bindingStore := devicebinding.NewStore(st.DB)
	if err := bindingStore.AutoMigrate(); err != nil {
		t.Fatalf("binding migrate: %v", err)
	}
	bindingService := devicebinding.NewService(bindingStore, devicebinding.Options{
		Now: func() time.Time { return time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC) },
	})
	if _, err := bindingService.EnsureDevice(t.Context(), devicebinding.DeviceSeed{
		DeviceID: "dev-001", BindingCode: "ANBAN-111111", DisplayName: "客厅安伴", ElderDisplayName: "王阿姨",
	}); err != nil {
		t.Fatalf("EnsureDevice: %v", err)
	}

	ms := mind.NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("mind migrate: %v", err)
	}
	return ms, accountService, bindingService, cleanup
}

func saveRouteSnapshotState(t *testing.T, ms *mind.Store, deviceID, theme string, warmth float64) {
	t.Helper()
	now := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	if err := ms.SaveSelfState(t.Context(), mind.SelfState{
		DeviceID: deviceID, At: now, Warmth: warmth, Concern: 0.7, Curiosity: 0.4,
		Playfulness: 0.3, Energy: 0.5, Quietness: 0.6, Patience: 0.7, Confidence: 0.8,
		FamilyWeight: 1,
	}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	if err := ms.SaveLifeState(t.Context(), mind.LifeState{
		DeviceID: deviceID, At: now, TodayTheme: theme, CareFocus: "轻轻留意",
	}); err != nil {
		t.Fatalf("SaveLifeState: %v", err)
	}
}

func registerAndBindMindRouteAccount(t *testing.T, r *gin.Engine, phone, role string) string {
	t.Helper()
	register := httptest.NewRecorder()
	r.ServeHTTP(register, mindRouteJSONRequest(http.MethodPost, "/api/auth/register", "{\"phone\":\""+phone+"\",\"password\":\"secret123\",\"nickname\":\"家人\"}"))
	token := extractMindRouteJSONField(t, register.Body.String(), "token")
	if token == "" {
		t.Fatalf("register body missing token: %s", register.Body.String())
	}
	req := mindRouteJSONRequest(http.MethodPost, "/api/device-binding", "{\"role\":\""+role+"\",\"bindingCode\":\"ANBAN-111111\"}")
	req.Header.Set("Authorization", "Bearer "+token)
	bind := httptest.NewRecorder()
	r.ServeHTTP(bind, req)
	if bind.Code != http.StatusCreated {
		t.Fatalf("bind status=%d body=%s", bind.Code, bind.Body.String())
	}
	return token
}

func mindRouteJSONRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func extractMindRouteJSONField(t *testing.T, body, field string) string {
	t.Helper()
	needle := "\"" + field + "\":\""
	start := strings.Index(body, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	end := strings.Index(body[start:], "\"")
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}
