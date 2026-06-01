package reminder

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
	return s.db.AutoMigrate(&Reminder{})
}

func (s *Store) Create(ctx context.Context, reminder *Reminder) error {
	return s.db.WithContext(ctx).Create(reminder).Error
}

func (s *Store) Update(ctx context.Context, reminder *Reminder) error {
	return s.db.WithContext(ctx).Save(reminder).Error
}

func (s *Store) List(ctx context.Context, filter ListFilter) ([]Reminder, error) {
	q := s.db.WithContext(ctx).Order("scheduled_at asc, id asc")
	if filter.DeviceID != "" {
		q = q.Where("device_id = ?", filter.DeviceID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var out []Reminder
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
