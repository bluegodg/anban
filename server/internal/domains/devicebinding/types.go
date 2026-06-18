package devicebinding

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput        = errors.New("devicebinding: invalid input")
	ErrDeviceNotFound      = errors.New("devicebinding: device not found")
	ErrAccountAlreadyBound = errors.New("devicebinding: account already bound")
	ErrAdminAlreadyBound   = errors.New("devicebinding: admin already bound")
	ErrNotBound            = errors.New("devicebinding: account not bound")
	ErrAdminRequired       = errors.New("devicebinding: admin required")
	ErrMemberNotFound      = errors.New("devicebinding: member not found")
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type AnbanDevice struct {
	ID                 uint   `gorm:"primaryKey"`
	DeviceID           string `gorm:"size:120;uniqueIndex;not null"`
	BindingCode        string `gorm:"size:40;uniqueIndex;not null"`
	BindingCodeVersion int    `gorm:"not null"`
	DisplayName        string `gorm:"size:100"`
	ElderDisplayName   string `gorm:"size:100"`
	BindingCodeResetAt time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DeviceBinding struct {
	ID            uint      `gorm:"primaryKey"`
	AccountID     uint      `gorm:"uniqueIndex;not null"`
	AnbanDeviceID uint      `gorm:"index;not null"`
	Role          Role      `gorm:"size:20;index;not null"`
	BoundAt       time.Time `gorm:"index;not null"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type DeviceSeed struct {
	DeviceID         string
	BindingCode      string
	DisplayName      string
	ElderDisplayName string
}

type BindRequest struct {
	AccountID   uint   `json:"-"`
	Role        Role   `json:"role"`
	BindingCode string `json:"bindingCode"`
}

type BindingView struct {
	BindingID         uint   `json:"bindingId"`
	DeviceRecordID    uint   `json:"-"`
	DeviceID          string `json:"deviceId"`
	DeviceDisplayName string `json:"deviceDisplayName"`
	ElderDisplayName  string `json:"elderDisplayName"`
	Role              Role   `json:"role"`
}

type MemberBinding struct {
	BindingID uint      `json:"bindingId"`
	AccountID uint      `json:"accountId"`
	Role      Role      `json:"role"`
	BoundAt   time.Time `json:"boundAt"`
}

type CodeResetResult struct {
	DeviceID    string `json:"deviceId"`
	BindingCode string `json:"bindingCode"`
}
