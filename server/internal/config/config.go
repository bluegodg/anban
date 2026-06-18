// Package config 从环境变量装载安伴后端运行所需配置。
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ManagerBaseURL           string // xiaozhi manager 根地址，如 http://localhost:8080
	ManagerAPIToken          string // manager 签发的 API Token（X-API-Token）
	DBDSN                    string // sqlite 文件路径
	AccessCode               string // 子女端访问码（简化登录）
	DevVerificationCode      string // 开发模式验证码
	DemoDeviceID             string // 默认可绑定设备的真实 deviceId
	DemoBindingCode          string // 默认可绑定设备的设备码
	DemoDeviceDisplayName    string
	DemoElderDisplayName     string
	ListenAddr               string   // 安伴 HTTP 监听地址
	AllowedOrigins           []string // 子女端 Web 允许跨域访问的来源
	LLM                      LLMConfig
	MemoryDistillCron        string
	VisionPresenceInterval   time.Duration
	MindLoopInterval         time.Duration
	MindHistoryInterval      time.Duration
	MindProactiveCooldown    time.Duration
	MindProactiveDaytimeOnly bool
	TimezoneName             string
	TimezoneLocation         *time.Location
}

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func (c LLMConfig) Enabled() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.Model != ""
}

func Load() (Config, error) {
	visionPresenceInterval, err := durationEnv("ANBAN_VISION_PRESENCE_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	mindLoopInterval, err := durationEnv("ANBAN_MIND_LOOP_INTERVAL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	mindHistoryInterval, err := durationEnv("ANBAN_MIND_HISTORY_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	mindProactiveCooldown, err := durationEnv("ANBAN_MIND_PROACTIVE_COOLDOWN", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	mindProactiveDaytimeOnly, err := boolEnv("ANBAN_MIND_PROACTIVE_DAYTIME_ONLY", true)
	if err != nil {
		return Config{}, err
	}
	timezoneName := envOr("ANBAN_TIMEZONE", "Asia/Shanghai")
	timezoneLocation, err := time.LoadLocation(timezoneName)
	if err != nil {
		log.Printf("config: ANBAN_TIMEZONE=%q 加载失败，回退 UTC: %v", timezoneName, err)
		timezoneName = "UTC"
		timezoneLocation = time.UTC
	}

	c := Config{
		ManagerBaseURL:           trimEnv("ANBAN_MANAGER_BASE_URL"),
		ManagerAPIToken:          trimEnv("ANBAN_MANAGER_API_TOKEN"),
		AccessCode:               trimEnv("ANBAN_ACCESS_CODE"),
		DevVerificationCode:      envOr("ANBAN_DEV_VERIFICATION_CODE", "123456"),
		DemoDeviceID:             envOr("ANBAN_DEMO_DEVICE_ID", "9c:13:9e:8b:af:28"),
		DemoBindingCode:          envOr("ANBAN_DEMO_BINDING_CODE", "ANBAN-482913"),
		DemoDeviceDisplayName:    envOr("ANBAN_DEMO_DEVICE_DISPLAY_NAME", "客厅安伴"),
		DemoElderDisplayName:     envOr("ANBAN_DEMO_ELDER_DISPLAY_NAME", "老人"),
		DBDSN:                    envOr("ANBAN_DB_DSN", "anban.db"),
		ListenAddr:               envOr("ANBAN_LISTEN_ADDR", ":8090"),
		AllowedOrigins:           splitCSV(envOr("ANBAN_ALLOWED_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173")),
		MemoryDistillCron:        envOr("ANBAN_MEMORY_DISTILL_CRON", "*/30 * * * *"),
		VisionPresenceInterval:   visionPresenceInterval,
		MindLoopInterval:         mindLoopInterval,
		MindHistoryInterval:      mindHistoryInterval,
		MindProactiveCooldown:    mindProactiveCooldown,
		MindProactiveDaytimeOnly: mindProactiveDaytimeOnly,
		TimezoneName:             timezoneName,
		TimezoneLocation:         timezoneLocation,
		LLM: LLMConfig{
			BaseURL: trimEnv("ANBAN_LLM_BASE_URL"),
			APIKey:  trimEnv("ANBAN_LLM_API_KEY"),
			Model:   trimEnv("ANBAN_LLM_MODEL"),
		},
	}
	if c.ManagerBaseURL == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_BASE_URL 必填")
	}
	if c.ManagerAPIToken == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 必填")
	}
	if IsPlaceholderValue(c.ManagerAPIToken) {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 不能使用示例占位值")
	}
	if c.AccessCode == "" {
		return Config{}, fmt.Errorf("config: ANBAN_ACCESS_CODE 必填")
	}
	return c, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	raw := trimEnv(key)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s 必须是 Go duration（如 30s、1m）: %w", key, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("config: %s 不能为负数", key)
	}
	if d > 0 && d < 10*time.Second {
		return 0, fmt.Errorf("config: %s 不能小于 10s，避免过度调度", key)
	}
	return d, nil
}

func boolEnv(key string, def bool) (bool, error) {
	raw := trimEnv(key)
	if raw == "" {
		return def, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("config: %s 必须是 bool（true/false）: %w", key, err)
	}
	return value, nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func trimEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func IsPlaceholderValue(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "请填") || strings.Contains(value, "<") || strings.Contains(value, ">")
}
