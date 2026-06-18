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

// ErrProactiveVoiceThrottled 表示同一设备主动语音 10 分钟共享配额已用完。
var ErrProactiveVoiceThrottled = errors.New("anban: proactive voice throttled")

// DeviceID 是 xiaozhi 侧的设备标识（= manager 的 device_name）。
type DeviceID string

// ProactiveVoiceGate 是 #2 问候、#6 提醒、#7 视觉触发共用的主动语音配额闸门。
type ProactiveVoiceGate interface {
	TryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (ProactiveVoiceLease, error)
}

// ProactiveVoiceLease 允许调用方在 xiaozhi 注入成功后提交，失败时回滚本次预占。
type ProactiveVoiceLease interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// ProactiveGreetingResult 是跨域返回给 vision/status 的最小问候结果摘要。
type ProactiveGreetingResult struct {
	ID           uint   `json:"greetingId,omitempty"`
	Status       string `json:"status"`
	Text         string `json:"text,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ProactiveGreetingTrigger 让 vision 触发一句主动问候，而不直接 import greeting 域。
type ProactiveGreetingTrigger interface {
	TriggerProactiveGreeting(ctx context.Context, deviceID string) (ProactiveGreetingResult, error)
}

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

type TimelineMessage struct {
	MessageID         uint
	Text              string
	SenderDisplayName string
	SenderRole        string
	AvatarColor       string
	Status            string
	QueuedAt          time.Time
	PlayedAt          *time.Time
}

type TimelineMessageReader interface {
	ListTimelineMessages(ctx context.Context, deviceID string, limit int) ([]TimelineMessage, error)
}

type ElderDisplayNameReader interface {
	GetElderDisplayName(ctx context.Context, deviceID string) (string, error)
}

const (
	GinContextAuthMode          = "anban.authMode"
	GinContextAccountID         = "anban.accountID"
	GinContextDeviceID          = "anban.deviceID"
	GinContextDeviceRole        = "anban.deviceRole"
	GinContextSenderDisplayName = "anban.senderDisplayName"
	GinContextSenderAvatarColor = "anban.senderAvatarColor"
	GinContextElderDisplayName  = "anban.elderDisplayName"
)
