// Package types 放跨模块共享、与具体业务域无关的小类型。
// 纪律：这里只允许放被两个及以上模块共用的类型；任何域专属类型放到该域自己的 types.go。
package types

import (
	"context"
	"errors"
	"time"
)

// ErrNotImplemented 供尚未实现的接口方法返回（地基期 FakeClient / 占位用）。
var ErrNotImplemented = errors.New("anban: not implemented")

// DeviceID 是 xiaozhi 侧的设备标识（= manager 的 device_name）。
type DeviceID string

// MessageStatusSummary 是 status 域展示留言播放状态所需的最小跨域摘要。
type MessageStatusSummary struct {
	MessageID uint       `json:"messageId"`
	Status    string     `json:"status"`
	QueuedAt  time.Time  `json:"queuedAt"`
	PlayedAt  *time.Time `json:"playedAt,omitempty"`
}

// MessageStatusReader 让 status 域读取留言状态摘要，而不直接 import message 域。
type MessageStatusReader interface {
	ListMessageStatusSummaries(ctx context.Context, deviceID string, limit int) ([]MessageStatusSummary, error)
}
