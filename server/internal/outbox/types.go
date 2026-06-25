// Package outbox 让"主动播报"对一台待命(UDP 冷)设备不再石沉大海。
//
// 背景：xiaozhi 固件对待命设备不回 speak_request 的 speak_ready，manager 的
// inject-message 却依旧返回 success:true（异步广播），所以 anban 无法在发送时
//察觉失败。本包把所有主动播报先入队，等设备"热"起来（最近有对话活动、处于
// speak_request 复用窗口内）再补播，从而保证子女留言/主动问候不丢。
//
// 纪律：只通过 xiaozhiclient.Client 南向投递，不碰 xiaozhi 源码（方案C）。
package outbox

import "time"

// Status 是一条待播条目的生命周期。
type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
	StatusExpired   Status = "expired"
)

// Item 是一条排队中的主动播报。
type Item struct {
	ID          uint   `gorm:"primaryKey"`
	DeviceID    string `gorm:"size:120;index:idx_outbox_device_status"`
	Text        string `gorm:"type:text"`
	SkipLLM     bool
	AutoListen  *bool
	Status      Status `gorm:"size:20;index:idx_outbox_device_status"`
	Attempts    int
	LastError   string `gorm:"type:text"`
	CreatedAt   time.Time
	ExpiresAt   time.Time `gorm:"index"`
	DeliveredAt *time.Time
}

// TableName 固定表名，避免 GORM 复数化歧义。
func (Item) TableName() string { return "proactive_outbox" }
