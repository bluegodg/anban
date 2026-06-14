package status

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
	return s.db.AutoMigrate(&SnapshotCache{})
}

func (s *Store) Upsert(ctx context.Context, cache *SnapshotCache) error {
	var existing SnapshotCache
	err := s.db.WithContext(ctx).Where("device_id = ?", cache.DeviceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(cache).Error
	}
	if err != nil {
		return err
	}

	cache.ID = existing.ID
	cache.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(cache).Error
}

func (s *Store) Get(ctx context.Context, deviceID string) (SnapshotCache, error) {
	var cache SnapshotCache
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&cache).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SnapshotCache{}, ErrNotFound
	}
	if err != nil {
		return SnapshotCache{}, err
	}
	return cache, nil
}
