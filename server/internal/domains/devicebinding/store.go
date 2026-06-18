package devicebinding

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
	if err := s.db.AutoMigrate(&AnbanDevice{}, &DeviceBinding{}); err != nil {
		return err
	}
	return s.db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_device_bindings_single_admin ON device_bindings(anban_device_id) WHERE role = 'admin'",
	).Error
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) GetDeviceByID(ctx context.Context, id uint) (AnbanDevice, error) {
	var device AnbanDevice
	err := s.db.WithContext(ctx).First(&device, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AnbanDevice{}, ErrDeviceNotFound
	}
	if err != nil {
		return AnbanDevice{}, err
	}
	return device, nil
}

func (s *Store) GetDeviceByDeviceID(ctx context.Context, deviceID string) (AnbanDevice, error) {
	var device AnbanDevice
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&device).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AnbanDevice{}, ErrDeviceNotFound
	}
	if err != nil {
		return AnbanDevice{}, err
	}
	return device, nil
}

func (s *Store) GetDeviceByBindingCode(ctx context.Context, code string) (AnbanDevice, error) {
	var device AnbanDevice
	err := s.db.WithContext(ctx).Where("binding_code = ?", code).First(&device).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AnbanDevice{}, ErrDeviceNotFound
	}
	if err != nil {
		return AnbanDevice{}, err
	}
	return device, nil
}

func (s *Store) CreateDevice(ctx context.Context, device *AnbanDevice) error {
	return s.db.WithContext(ctx).Create(device).Error
}

func (s *Store) UpdateDevice(ctx context.Context, device *AnbanDevice) error {
	return s.db.WithContext(ctx).Save(device).Error
}

func (s *Store) GetBindingByAccount(ctx context.Context, accountID uint) (DeviceBinding, error) {
	var binding DeviceBinding
	err := s.db.WithContext(ctx).Where("account_id = ?", accountID).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeviceBinding{}, ErrNotBound
	}
	if err != nil {
		return DeviceBinding{}, err
	}
	return binding, nil
}

func (s *Store) CountAdminBindings(ctx context.Context, deviceID uint) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&DeviceBinding{}).
		Where("anban_device_id = ? AND role = ?", deviceID, RoleAdmin).
		Count(&count).Error
	return count, err
}

func (s *Store) CreateBinding(ctx context.Context, binding *DeviceBinding) error {
	return s.db.WithContext(ctx).Create(binding).Error
}

func (s *Store) DeleteBinding(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&DeviceBinding{}, id).Error
}

func (s *Store) ListMembers(ctx context.Context, deviceRecordID uint) ([]DeviceBinding, error) {
	var bindings []DeviceBinding
	if err := s.db.WithContext(ctx).
		Where("anban_device_id = ? AND role = ?", deviceRecordID, RoleMember).
		Order("bound_at asc, id asc").
		Find(&bindings).Error; err != nil {
		return nil, err
	}
	return bindings, nil
}

func (s *Store) GetMember(ctx context.Context, deviceRecordID uint, memberAccountID uint) (DeviceBinding, error) {
	var binding DeviceBinding
	err := s.db.WithContext(ctx).
		Where("anban_device_id = ? AND account_id = ? AND role = ?", deviceRecordID, memberAccountID, RoleMember).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeviceBinding{}, ErrMemberNotFound
	}
	if err != nil {
		return DeviceBinding{}, err
	}
	return binding, nil
}
