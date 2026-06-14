package message

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
	return s.db.AutoMigrate(&Message{})
}

func (s *Store) Create(ctx context.Context, msg *Message) error {
	return s.db.WithContext(ctx).Create(msg).Error
}

func (s *Store) Update(ctx context.Context, msg *Message) error {
	return s.db.WithContext(ctx).Save(msg).Error
}

func (s *Store) Get(ctx context.Context, id uint) (Message, error) {
	var msg Message
	err := s.db.WithContext(ctx).First(&msg, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Message{}, ErrNotFound
	}
	if err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (s *Store) List(ctx context.Context, filter ListFilter) ([]Message, error) {
	q := s.db.WithContext(ctx).Order("queued_at desc, id desc")
	if filter.DeviceID != "" {
		q = q.Where("device_id = ?", filter.DeviceID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	var out []Message
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
