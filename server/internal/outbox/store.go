package outbox

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Store 持久化待播队列。
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Item{})
}

// Enqueue 落一条待播。
func (s *Store) Enqueue(ctx context.Context, item *Item) error {
	return s.db.WithContext(ctx).Create(item).Error
}

// CountPending 返回某设备当前待播数量。
func (s *Store) CountPending(ctx context.Context, deviceID string) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&Item{}).
		Where("device_id = ? AND status = ?", deviceID, StatusPending).
		Count(&n).Error
	return n, err
}

// ListPending 取某设备待播条目（最旧在前），limit<=0 表示不限。
func (s *Store) ListPending(ctx context.Context, deviceID string, limit int) ([]Item, error) {
	var out []Item
	q := s.db.WithContext(ctx).
		Where("device_id = ? AND status = ?", deviceID, StatusPending).
		Order("created_at asc, id asc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&out).Error
	return out, err
}

// PendingDeviceIDs 返回所有还有待播条目的设备。
func (s *Store) PendingDeviceIDs(ctx context.Context) ([]string, error) {
	var out []string
	err := s.db.WithContext(ctx).Model(&Item{}).
		Where("status = ?", StatusPending).
		Distinct().
		Pluck("device_id", &out).Error
	return out, err
}

// Save 覆盖保存一条（用于记录投递失败的重试计数等）。
func (s *Store) Save(ctx context.Context, item *Item) error {
	return s.db.WithContext(ctx).Save(item).Error
}

// MarkDelivered 标记已投递。
func (s *Store) MarkDelivered(ctx context.Context, item *Item, at time.Time) error {
	at = at.UTC()
	item.Status = StatusDelivered
	item.DeliveredAt = &at
	item.LastError = ""
	return s.db.WithContext(ctx).Save(item).Error
}

// ExpirePending 把某设备已过期的待播标记为 expired，返回处理条数。
func (s *Store) ExpirePending(ctx context.Context, deviceID string, now time.Time) (int, error) {
	res := s.db.WithContext(ctx).Model(&Item{}).
		Where("device_id = ? AND status = ? AND expires_at <= ?", deviceID, StatusPending, now.UTC()).
		Update("status", StatusExpired)
	return int(res.RowsAffected), res.Error
}
