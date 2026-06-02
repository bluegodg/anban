package greeting

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
	return s.db.AutoMigrate(&Greeting{}, &GreetingSchedule{})
}

func (s *Store) Create(ctx context.Context, greeting *Greeting) error {
	return s.db.WithContext(ctx).Create(greeting).Error
}

func (s *Store) Update(ctx context.Context, greeting *Greeting) error {
	return s.db.WithContext(ctx).Save(greeting).Error
}

func (s *Store) UpsertSchedule(ctx context.Context, schedule *GreetingSchedule) error {
	var existing GreetingSchedule
	err := s.db.WithContext(ctx).Where("device_id = ?", schedule.DeviceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(schedule).Error
	}
	if err != nil {
		return err
	}

	schedule.ID = existing.ID
	schedule.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(schedule).Error
}

func (s *Store) GetSchedule(ctx context.Context, deviceID string) (GreetingSchedule, error) {
	var schedule GreetingSchedule
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&schedule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GreetingSchedule{}, ErrNotFound
	}
	if err != nil {
		return GreetingSchedule{}, err
	}
	return schedule, nil
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
