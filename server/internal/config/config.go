// Package config 从环境变量装载安伴后端运行所需配置。
package config

import (
	"fmt"
	"os"
)

type Config struct {
	ManagerBaseURL  string // xiaozhi manager 根地址，如 http://localhost:8080
	ManagerAPIToken string // manager 签发的 API Token（X-API-Token）
	DBDSN           string // sqlite 文件路径
	AccessCode      string // 子女端访问码（简化登录）
	ListenAddr      string // 安伴 HTTP 监听地址
}

func Load() (Config, error) {
	c := Config{
		ManagerBaseURL:  os.Getenv("ANBAN_MANAGER_BASE_URL"),
		ManagerAPIToken: os.Getenv("ANBAN_MANAGER_API_TOKEN"),
		AccessCode:      os.Getenv("ANBAN_ACCESS_CODE"),
		DBDSN:           envOr("ANBAN_DB_DSN", "anban.db"),
		ListenAddr:      envOr("ANBAN_LISTEN_ADDR", ":8090"),
	}
	if c.ManagerBaseURL == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_BASE_URL 必填")
	}
	if c.ManagerAPIToken == "" {
		return Config{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 必填")
	}
	if c.AccessCode == "" {
		return Config{}, fmt.Errorf("config: ANBAN_ACCESS_CODE 必填")
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
