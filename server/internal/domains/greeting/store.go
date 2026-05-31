package greeting

import (
	"context"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Greeting{})
}

func (s *Store) Create(ctx context.Context, greeting *Greeting) error {
	return s.db.WithContext(ctx).Create(greeting).Error
}

func (s *Store) Update(ctx context.Context, greeting *Greeting) error {
	return s.db.WithContext(ctx).Save(greeting).Error
}

func (s *Store) List(ctx context.Context, filter ListFilter) ([]Greeting, error) {
	q := s.db.WithContext(ctx).Order("triggered_at desc, id desc")
	if filter.DeviceID != "" {
		q = q.Where("device_id = ?", filter.DeviceID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var out []Greeting
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
