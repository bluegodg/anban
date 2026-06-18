package config

import (
	"reflect"
	"testing"
	"time"
)

func TestLoadFailsWhenManagerTokenMissing(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when ANBAN_MANAGER_API_TOKEN missing, got nil")
	}
}

func TestLoadRejectsExampleManagerTokenPlaceholder(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "请填_manager签发的APIToken")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected example manager token placeholder to be rejected")
	}
}

func TestLoadOKWithDefaults(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ListenAddr != ":8090" {
		t.Fatalf("ListenAddr default = %q, want :8090", c.ListenAddr)
	}
	if c.DBDSN != "anban.db" {
		t.Fatalf("DBDSN default = %q, want anban.db", c.DBDSN)
	}
	wantOrigins := []string{"http://127.0.0.1:5173", "http://localhost:5173"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
	if c.LLM.Enabled() {
		t.Fatal("LLM.Enabled() = true with no ANBAN_LLM_* env; want profile-only fallback")
	}
	if c.VisionPresenceInterval != 30*time.Second {
		t.Fatalf("VisionPresenceInterval default = %s, want 30s", c.VisionPresenceInterval)
	}
	if c.VisionCaptureTimeout != 30*time.Second {
		t.Fatalf("VisionCaptureTimeout default = %s, want 30s", c.VisionCaptureTimeout)
	}
	if c.VisionRetentionDays != 30 {
		t.Fatalf("VisionRetentionDays default = %d, want 30", c.VisionRetentionDays)
	}
	if c.VisionMaxCapturesPerDevice != 100 {
		t.Fatalf("VisionMaxCapturesPerDevice default = %d, want 100", c.VisionMaxCapturesPerDevice)
	}
	if c.MindLoopInterval != 15*time.Minute {
		t.Fatalf("MindLoopInterval default = %s, want 15m", c.MindLoopInterval)
	}
	if c.MindHistoryInterval != time.Minute {
		t.Fatalf("MindHistoryInterval default = %s, want 1m", c.MindHistoryInterval)
	}
	if c.MindProactiveCooldown != 30*time.Minute {
		t.Fatalf("MindProactiveCooldown default = %s, want 30m", c.MindProactiveCooldown)
	}
	if !c.MindProactiveDaytimeOnly {
		t.Fatal("MindProactiveDaytimeOnly default = false, want true")
	}
	if c.TimezoneName != "Asia/Shanghai" || c.TimezoneLocation == nil {
		t.Fatalf("timezone = %q/%v, want Asia/Shanghai location", c.TimezoneName, c.TimezoneLocation)
	}
}

func TestLoadParsesVisionLookConfig(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_VISION_MEDIA_ROOT", " /data/anban-media ")
	t.Setenv("ANBAN_DEVICE_VISION_TOKEN", " device-secret ")
	t.Setenv("ANBAN_XIAOZHI_VISION_URL", " http://127.0.0.1:8989/xiaozhi/api/vision ")
	t.Setenv("ANBAN_VISION_CAPTURE_TIMEOUT", "45s")
	t.Setenv("ANBAN_VISION_RETENTION_DAYS", "14")
	t.Setenv("ANBAN_VISION_MAX_CAPTURES_PER_DEVICE", "25")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.VisionMediaRoot != "/data/anban-media" {
		t.Fatalf("VisionMediaRoot = %q, want trimmed media root", c.VisionMediaRoot)
	}
	if c.DeviceVisionToken != "device-secret" {
		t.Fatalf("DeviceVisionToken = %q, want trimmed token", c.DeviceVisionToken)
	}
	if c.XiaozhiVisionURL != "http://127.0.0.1:8989/xiaozhi/api/vision" {
		t.Fatalf("XiaozhiVisionURL = %q, want trimmed URL", c.XiaozhiVisionURL)
	}
	if c.VisionCaptureTimeout != 45*time.Second {
		t.Fatalf("VisionCaptureTimeout = %s, want 45s", c.VisionCaptureTimeout)
	}
	if c.VisionRetentionDays != 14 {
		t.Fatalf("VisionRetentionDays = %d, want 14", c.VisionRetentionDays)
	}
	if c.VisionMaxCapturesPerDevice != 25 {
		t.Fatalf("VisionMaxCapturesPerDevice = %d, want 25", c.VisionMaxCapturesPerDevice)
	}
}

func TestLoadParsesOptionalLLMConfig(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_LLM_BASE_URL", " https://ark.cn-beijing.volces.com/api/v3 ")
	t.Setenv("ANBAN_LLM_API_KEY", " ark_key ")
	t.Setenv("ANBAN_LLM_MODEL", " doubao-seed ")
	t.Setenv("ANBAN_MEMORY_DISTILL_CRON", "*/30 * * * *")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.LLM.Enabled() {
		t.Fatal("LLM.Enabled() = false, want true when base URL, key and model are present")
	}
	if c.LLM.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" {
		t.Fatalf("LLM.BaseURL = %q, want trimmed Ark base URL", c.LLM.BaseURL)
	}
	if c.LLM.APIKey != "ark_key" || c.LLM.Model != "doubao-seed" {
		t.Fatalf("LLM config = %+v, want trimmed key/model", c.LLM)
	}
	if c.MemoryDistillCron != "*/30 * * * *" {
		t.Fatalf("MemoryDistillCron = %q, want env value", c.MemoryDistillCron)
	}
}

func TestLoadParsesOptionalMemoryProviderToken(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_MEMORY_PROVIDER_TOKEN", " 0123456789abcdef0123456789abcdef ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.MemoryProviderToken != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("MemoryProviderToken = %q, want trimmed token", c.MemoryProviderToken)
	}
}

func TestLoadRejectsWeakMemoryProviderToken(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")

	for _, token := range []string{"short-token", "<请填随机令牌>"} {
		t.Run(token, func(t *testing.T) {
			t.Setenv("ANBAN_MEMORY_PROVIDER_TOKEN", token)
			if _, err := Load(); err == nil {
				t.Fatalf("expected weak provider token %q to be rejected", token)
			}
		})
	}
}

func TestLoadParsesAllowedOrigins(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_ALLOWED_ORIGINS", "http://child.local:5173, https://demo.example ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"http://child.local:5173", "https://demo.example"}
	if !reflect.DeepEqual(c.AllowedOrigins, want) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, want)
	}
}

func TestLoadParsesVisionPresenceInterval(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_VISION_PRESENCE_INTERVAL", "45s")
	t.Setenv("ANBAN_MIND_LOOP_INTERVAL", "20m")
	t.Setenv("ANBAN_MIND_HISTORY_INTERVAL", "90s")
	t.Setenv("ANBAN_MIND_PROACTIVE_COOLDOWN", "45m")
	t.Setenv("ANBAN_MIND_PROACTIVE_DAYTIME_ONLY", "false")
	t.Setenv("ANBAN_TIMEZONE", "UTC")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.VisionPresenceInterval != 45*time.Second {
		t.Fatalf("VisionPresenceInterval = %s, want 45s", c.VisionPresenceInterval)
	}
	if c.MindLoopInterval != 20*time.Minute {
		t.Fatalf("MindLoopInterval = %s, want 20m", c.MindLoopInterval)
	}
	if c.MindHistoryInterval != 90*time.Second {
		t.Fatalf("MindHistoryInterval = %s, want 90s", c.MindHistoryInterval)
	}
	if c.MindProactiveCooldown != 45*time.Minute {
		t.Fatalf("MindProactiveCooldown = %s, want 45m", c.MindProactiveCooldown)
	}
	if c.MindProactiveDaytimeOnly {
		t.Fatal("MindProactiveDaytimeOnly = true, want false")
	}
	if c.TimezoneName != "UTC" || c.TimezoneLocation != time.UTC {
		t.Fatalf("timezone = %q/%v, want UTC", c.TimezoneName, c.TimezoneLocation)
	}
}

func TestLoadRejectsInvalidMindProactiveDaytimeOnly(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_MIND_PROACTIVE_DAYTIME_ONLY", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid ANBAN_MIND_PROACTIVE_DAYTIME_ONLY to be rejected")
	}
}

func TestLoadFallsBackToUTCForInvalidTimezone(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_TIMEZONE", "Mars/Olympus")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.TimezoneName != "UTC" || c.TimezoneLocation != time.UTC {
		t.Fatalf("timezone = %q/%v, want UTC fallback", c.TimezoneName, c.TimezoneLocation)
	}
}

func TestLoadRejectsTooFrequentVisionPresenceInterval(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_VISION_PRESENCE_INTERVAL", "5s")

	if _, err := Load(); err == nil {
		t.Fatal("expected ANBAN_VISION_PRESENCE_INTERVAL below 10s to be rejected")
	}
}

func TestLoadTrimsOptionalEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_DB_DSN", " ./data/anban.db ")
	t.Setenv("ANBAN_LISTEN_ADDR", " :8091 ")
	t.Setenv("ANBAN_ALLOWED_ORIGINS", " http://127.0.0.1:5173 , http://localhost:5173 ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.DBDSN != "./data/anban.db" {
		t.Fatalf("DBDSN = %q, want trimmed value", c.DBDSN)
	}
	if c.ListenAddr != ":8091" {
		t.Fatalf("ListenAddr = %q, want trimmed value", c.ListenAddr)
	}
	wantOrigins := []string{"http://127.0.0.1:5173", "http://localhost:5173"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
}

func TestLoadTrimsRequiredEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", " http://localhost:8080/ ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", " tok_123 ")
	t.Setenv("ANBAN_ACCESS_CODE", " demo ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ManagerBaseURL != "http://localhost:8080/" {
		t.Fatalf("ManagerBaseURL = %q, want trimmed URL", c.ManagerBaseURL)
	}
	if c.ManagerAPIToken != "tok_123" {
		t.Fatalf("ManagerAPIToken = %q, want trimmed token", c.ManagerAPIToken)
	}
	if c.AccessCode != "demo" {
		t.Fatalf("AccessCode = %q, want trimmed access code", c.AccessCode)
	}
}

func TestLoadRejectsWhitespaceRequiredEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "   ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected whitespace manager URL to be rejected")
	}
}
