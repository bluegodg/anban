package account

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Account{}, &AuthSession{}, &VerificationCode{})
}

func (s *Store) CreateAccount(ctx context.Context, account *Account) error {
	return s.db.WithContext(ctx).Create(account).Error
}

func (s *Store) UpdateAccount(ctx context.Context, account *Account) error {
	return s.db.WithContext(ctx).Save(account).Error
}

func (s *Store) GetByID(ctx context.Context, id uint) (Account, error) {
	var account Account
	err := s.db.WithContext(ctx).First(&account, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Store) GetByPhone(ctx context.Context, phone string) (Account, error) {
	var account Account
	err := s.db.WithContext(ctx).Where("phone = ?", phone).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Store) CreateSession(ctx context.Context, session *AuthSession) error {
	return s.db.WithContext(ctx).Create(session).Error
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (AuthSession, error) {
	var session AuthSession
	err := s.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AuthSession{}, ErrUnauthorized
	}
	if err != nil {
		return AuthSession{}, err
	}
	return session, nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash string, revokedAt any) error {
	return s.db.WithContext(ctx).
		Model(&AuthSession{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", revokedAt).Error
}

func (s *Store) CreateVerificationCode(ctx context.Context, code *VerificationCode) error {
	return s.db.WithContext(ctx).Create(code).Error
}

func (s *Store) LatestVerificationCode(ctx context.Context, phone, purpose string) (VerificationCode, error) {
	var code VerificationCode
	err := s.db.WithContext(ctx).
		Where("phone = ? AND purpose = ? AND consumed_at IS NULL", phone, purpose).
		Order("created_at desc, id desc").
		First(&code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VerificationCode{}, ErrInvalidVerificationCode
	}
	if err != nil {
		return VerificationCode{}, err
	}
	return code, nil
}

func (s *Store) ConsumeVerificationCode(ctx context.Context, id uint, consumedAt any) error {
	return s.db.WithContext(ctx).
		Model(&VerificationCode{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Update("consumed_at", consumedAt).Error
}
