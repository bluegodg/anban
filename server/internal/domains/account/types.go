package account

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput            = errors.New("account: invalid input")
	ErrDuplicatePhone          = errors.New("account: duplicate phone")
	ErrInvalidCredentials      = errors.New("account: invalid credentials")
	ErrUnauthorized            = errors.New("account: unauthorized")
	ErrNotFound                = errors.New("account: not found")
	ErrInvalidVerificationCode = errors.New("account: invalid verification code")
)

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "active"
)

type Account struct {
	ID                  uint          `gorm:"primaryKey" json:"accountId"`
	Phone               string        `gorm:"size:32;uniqueIndex;not null" json:"-"`
	PasswordHash        string        `gorm:"size:160" json:"-"`
	Nickname            string        `gorm:"size:80" json:"nickname"`
	RealName            string        `gorm:"size:80" json:"realName,omitempty"`
	RelationshipToElder string        `gorm:"size:80" json:"relationshipToElder,omitempty"`
	AvatarURL           string        `gorm:"size:255" json:"avatarUrl,omitempty"`
	AvatarColor         string        `gorm:"size:32" json:"avatarColor,omitempty"`
	Status              AccountStatus `gorm:"size:20;index;not null" json:"status"`
	CreatedAt           time.Time     `json:"-"`
	UpdatedAt           time.Time     `json:"-"`
}

type AuthSession struct {
	ID        uint      `gorm:"primaryKey"`
	AccountID uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"size:80;uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
	RevokedAt *time.Time `gorm:"index"`
}

type VerificationCode struct {
	ID         uint       `gorm:"primaryKey"`
	Phone      string     `gorm:"size:32;index;not null"`
	CodeHash   string     `gorm:"size:80;not null"`
	Purpose    string     `gorm:"size:40;index;not null"`
	ExpiresAt  time.Time  `gorm:"index;not null"`
	ConsumedAt *time.Time `gorm:"index"`
	CreatedAt  time.Time
}

type RegisterRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type LoginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type VerificationCodeRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

type VerificationCodeResponse struct {
	Sent      bool   `json:"sent"`
	DebugCode string `json:"debugCode,omitempty"`
}

type CodeLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type UpdateProfileRequest struct {
	Nickname            string `json:"nickname"`
	RealName            string `json:"realName"`
	RelationshipToElder string `json:"relationshipToElder"`
	AvatarURL           string `json:"avatarUrl"`
	AvatarColor         string `json:"avatarColor"`
}

type PublicAccount struct {
	AccountID           uint   `json:"accountId"`
	Phone               string `json:"phone"`
	Nickname            string `json:"nickname"`
	RealName            string `json:"realName,omitempty"`
	RelationshipToElder string `json:"relationshipToElder,omitempty"`
	AvatarURL           string `json:"avatarUrl,omitempty"`
	AvatarColor         string `json:"avatarColor,omitempty"`
	DisplayName         string `json:"displayName"`
}

type AuthResponse struct {
	Token   string        `json:"token"`
	Account PublicAccount `json:"account"`
}
