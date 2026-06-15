// Package config 从环境变量装载安伴后端运行所需配置。
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ManagerBaseURL    string   // xiaozhi manager 根地址，如 http://localhost:8080
	ManagerAPIToken   string   // manager 签发的 API Token（X-API-Token）
	DBDSN             string   // sqlite 文件路径
	AccessCode        string   // 子女端访问码（简化登录）
	ListenAddr        string   // 安伴 HTTP 监听地址
	AllowedOrigins    []string // 子女端 Web 允许跨域访问的来源
	LLM               LLMConfig
	MemoryDistillCron string
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
	c := Config{
		ManagerBaseURL:    trimEnv("ANBAN_MANAGER_BASE_URL"),
		ManagerAPIToken:   trimEnv("ANBAN_MANAGER_API_TOKEN"),
		AccessCode:        trimEnv("ANBAN_ACCESS_CODE"),
		DBDSN:             envOr("ANBAN_DB_DSN", "anban.db"),
		ListenAddr:        envOr("ANBAN_LISTEN_ADDR", ":8090"),
		AllowedOrigins:    splitCSV(envOr("ANBAN_ALLOWED_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173")),
		MemoryDistillCron: envOr("ANBAN_MEMORY_DISTILL_CRON", "*/30 * * * *"),
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
