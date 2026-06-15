package profile

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
	return s.db.AutoMigrate(&Profile{})
}

func (s *Store) Upsert(ctx context.Context, profile *Profile) error {
	var existing Profile
	err := s.db.WithContext(ctx).Where("device_id = ?", profile.DeviceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(profile).Error
	}
	if err != nil {
		return err
	}

	profile.ID = existing.ID
	profile.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(profile).Error
}

func (s *Store) Get(ctx context.Context, deviceID string) (Profile, error) {
	var profile Profile
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s *Store) ListDeviceIDs(ctx context.Context) ([]string, error) {
	var deviceIDs []string
	if err := s.db.WithContext(ctx).Model(&Profile{}).Order("updated_at desc").Pluck("device_id", &deviceIDs).Error; err != nil {
		return nil, err
	}
	return deviceIDs, nil
}
