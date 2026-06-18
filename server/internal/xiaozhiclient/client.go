// Package xiaozhiclient 是安伴唯一懂 xiaozhi 的地方：封装 manager 的 OpenAPI(/api/open/v1, X-API-Token)。
// 纪律：只有本包 import 网络/manager 细节；它不反向 import 任何 domain。
package xiaozhiclient

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrDeviceOffline      = errors.New("xiaozhi: device offline")
	ErrMCPToolUnavailable = errors.New("xiaozhi: mcp tool unavailable")
	ErrUpstreamTimeout    = errors.New("xiaozhi: upstream timeout")
)

// Client 是各业务域唯一可见的南向接口（域只依赖它，不碰 HTTP 细节）。
type Client interface {
	// InjectSpeak 让指定设备说一段话（主动播报）。message/reminder/greeting 用。
	InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error
	// GetDeviceStatus 读设备在线/最近互动。status 域用。
	GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error)
	// GetHistory 读近 N 条对话历史（只读）。status / 子女端深度用。
	GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error)
	// SetRolePrompt 把家庭画像写成 xiaozhi 人设 prompt。profile 域用。
	SetRolePrompt(ctx context.Context, deviceID, prompt string) error
	// CallDeviceMCPTool 远程调设备已注册的 MCP 工具（如拍照）。vision 域用。
	CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error)
}

// InjectOptions 对应 manager inject-message 的可选参数。
type InjectOptions struct {
	SkipLLM    bool  // true=直接念原话；false=过 LLM 润色
	AutoListen *bool // 非 nil 时控制"播完是否自动续听"；nil=用服务端默认
}

type DeviceStatus struct {
	DeviceID     string
	Online       bool
	LastActiveAt time.Time
}

type HistoryMessage struct {
	Role string // "user" | "assistant"
	Text string
	At   time.Time
}
