package outbox

import (
	"context"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

// Client 是 xiaozhiclient.Client 的装饰器：把主动播报(InjectSpeak)改为入队，
// 其余方法（状态/历史/角色 prompt/设备 MCP）原样透传给内层真实 client。
//
// 这样 greeting/message/reminder 三个域无需改动即可获得"冷设备不丢、热了补播"。
type Client struct {
	xiaozhiclient.Client // 内嵌真实 client：自动透传非 InjectSpeak 方法
	svc                  *Service
}

func NewClient(inner xiaozhiclient.Client, svc *Service) *Client {
	return &Client{Client: inner, svc: svc}
}

// InjectSpeak 不直接下发，而是入队，等设备活跃时由补播轮询投递。
func (c *Client) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	return c.svc.Enqueue(ctx, deviceID, text, opts)
}
