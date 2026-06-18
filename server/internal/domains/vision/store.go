package vision

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	if err := s.db.AutoMigrate(&Capture{}); err != nil {
		return err
	}
	return s.db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_vision_captures_pending_device ON vision_captures(device_id) WHERE status = 'pending'",
	).Error
}

func (s *Store) CreateCapture(ctx context.Context, capture *Capture) error {
	return s.db.WithContext(ctx).Create(capture).Error
}

func (s *Store) UpdateCapture(ctx context.Context, capture *Capture) error {
	return s.db.WithContext(ctx).Save(capture).Error
}

func (s *Store) GetCapture(ctx context.Context, deviceID, captureID string) (Capture, error) {
	var capture Capture
	err := s.db.WithContext(ctx).
		Where("device_id = ? AND capture_id = ?", deviceID, captureID).
		First(&capture).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Capture{}, ErrNotFound
	}
	if err != nil {
		return Capture{}, err
	}
	return capture, nil
}

func (s *Store) FindPendingCapture(ctx context.Context, deviceID string) (Capture, error) {
	var capture Capture
	err := s.db.WithContext(ctx).
		Where("device_id = ? AND status = ?", deviceID, CaptureStatusPending).
		Order("created_at desc, id desc").
		First(&capture).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Capture{}, ErrNotFound
	}
	if err != nil {
		return Capture{}, err
	}
	return capture, nil
}

func (s *Store) ListCaptures(ctx context.Context, filter CaptureListRequest) ([]Capture, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := s.db.WithContext(ctx).
		Where("device_id = ?", filter.DeviceID).
		Order("created_at desc, id desc").
		Limit(limit)
	var captures []Capture
	if err := q.Find(&captures).Error; err != nil {
		return nil, err
	}
	return captures, nil
}

func (s *Store) ListPendingCapturesCreatedBefore(ctx context.Context, cutoff time.Time) ([]Capture, error) {
	var captures []Capture
	if err := s.db.WithContext(ctx).
		Where("status = ? AND created_at <= ?", CaptureStatusPending, cutoff).
		Order("created_at asc, id asc").
		Find(&captures).Error; err != nil {
		return nil, err
	}
	return captures, nil
}

func (s *Store) ListCapturesExpiredBefore(ctx context.Context, now time.Time) ([]Capture, error) {
	var captures []Capture
	if err := s.db.WithContext(ctx).
		Where("status IN ? AND expires_at <= ?", []CaptureStatus{CaptureStatusSucceeded, CaptureStatusPartial}, now).
		Order("expires_at asc, id asc").
		Find(&captures).Error; err != nil {
		return nil, err
	}
	return captures, nil
}

func (s *Store) ListDeviceIDsWithActiveCaptures(ctx context.Context) ([]string, error) {
	var deviceIDs []string
	if err := s.db.WithContext(ctx).
		Model(&Capture{}).
		Where("status IN ?", []CaptureStatus{CaptureStatusSucceeded, CaptureStatusPartial}).
		Distinct().
		Pluck("device_id", &deviceIDs).Error; err != nil {
		return nil, err
	}
	return deviceIDs, nil
}

func (s *Store) ListActiveCapturesBeyondLimit(ctx context.Context, deviceID string, limit int) ([]Capture, error) {
	var captures []Capture
	if err := s.db.WithContext(ctx).
		Where("device_id = ? AND status IN ?", deviceID, []CaptureStatus{CaptureStatusSucceeded, CaptureStatusPartial}).
		Order("created_at desc, id desc").
		Offset(limit).
		Find(&captures).Error; err != nil {
		return nil, err
	}
	return captures, nil
}
