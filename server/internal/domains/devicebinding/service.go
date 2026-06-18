package devicebinding

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type Options struct {
	Now           func() time.Time
	CodeGenerator func() (string, error)
}

type Service struct {
	store         *Store
	now           func() time.Time
	codeGenerator func() (string, error)
}

func NewService(store *Store, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	codeGenerator := opts.CodeGenerator
	if codeGenerator == nil {
		codeGenerator = generateBindingCode
	}
	return &Service{store: store, now: now, codeGenerator: codeGenerator}
}

func (s *Service) EnsureDevice(ctx context.Context, seed DeviceSeed) (AnbanDevice, error) {
	deviceID := strings.TrimSpace(seed.DeviceID)
	bindingCode := strings.TrimSpace(seed.BindingCode)
	if deviceID == "" || bindingCode == "" {
		return AnbanDevice{}, ErrInvalidInput
	}
	device, err := s.store.GetDeviceByDeviceID(ctx, deviceID)
	if err == nil {
		changed := false
		if strings.TrimSpace(device.BindingCode) == "" {
			device.BindingCode = bindingCode
			changed = true
		}
		if seed.DisplayName != "" && device.DisplayName != seed.DisplayName {
			device.DisplayName = seed.DisplayName
			changed = true
		}
		if seed.ElderDisplayName != "" && device.ElderDisplayName != seed.ElderDisplayName {
			device.ElderDisplayName = seed.ElderDisplayName
			changed = true
		}
		if changed {
			if err := s.store.UpdateDevice(ctx, &device); err != nil {
				return AnbanDevice{}, err
			}
		}
		return device, nil
	}
	if err != ErrDeviceNotFound {
		return AnbanDevice{}, err
	}
	now := s.now().UTC()
	device = AnbanDevice{
		DeviceID:           deviceID,
		BindingCode:        bindingCode,
		BindingCodeVersion: 1,
		DisplayName:        strings.TrimSpace(seed.DisplayName),
		ElderDisplayName:   strings.TrimSpace(seed.ElderDisplayName),
		BindingCodeResetAt: now,
	}
	if err := s.store.CreateDevice(ctx, &device); err != nil {
		return AnbanDevice{}, err
	}
	return device, nil
}

func (s *Service) Bind(ctx context.Context, req BindRequest) (BindingView, error) {
	if req.AccountID == 0 || !validRole(req.Role) {
		return BindingView{}, ErrInvalidInput
	}
	code := strings.TrimSpace(req.BindingCode)
	if code == "" {
		return BindingView{}, ErrInvalidInput
	}
	if _, err := s.store.GetBindingByAccount(ctx, req.AccountID); err == nil {
		return BindingView{}, ErrAccountAlreadyBound
	} else if err != ErrNotBound {
		return BindingView{}, err
	}
	device, err := s.store.GetDeviceByBindingCode(ctx, code)
	if err != nil {
		return BindingView{}, err
	}
	if req.Role == RoleAdmin {
		count, err := s.store.CountAdminBindings(ctx, device.ID)
		if err != nil {
			return BindingView{}, err
		}
		if count > 0 {
			return BindingView{}, ErrAdminAlreadyBound
		}
	}
	now := s.now().UTC()
	binding := DeviceBinding{
		AccountID:     req.AccountID,
		AnbanDeviceID: device.ID,
		Role:          req.Role,
		BoundAt:       now,
	}
	if err := s.store.CreateBinding(ctx, &binding); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			if req.Role == RoleAdmin {
				return BindingView{}, ErrAdminAlreadyBound
			}
			return BindingView{}, ErrAccountAlreadyBound
		}
		return BindingView{}, err
	}
	return bindingView(binding, device), nil
}

func (s *Service) CurrentBinding(ctx context.Context, accountID uint) (BindingView, error) {
	if accountID == 0 {
		return BindingView{}, ErrInvalidInput
	}
	binding, err := s.store.GetBindingByAccount(ctx, accountID)
	if err != nil {
		return BindingView{}, err
	}
	device, err := s.store.GetDeviceByID(ctx, binding.AnbanDeviceID)
	if err != nil {
		return BindingView{}, err
	}
	return bindingView(binding, device), nil
}

func (s *Service) UnbindAdmin(ctx context.Context, accountID uint) error {
	binding, _, err := s.adminBinding(ctx, accountID)
	if err != nil {
		return err
	}
	return s.store.DeleteBinding(ctx, binding.ID)
}

func (s *Service) ResetBindingCode(ctx context.Context, accountID uint) (CodeResetResult, error) {
	_, device, err := s.adminBinding(ctx, accountID)
	if err != nil {
		return CodeResetResult{}, err
	}
	code, err := s.codeGenerator()
	if err != nil {
		return CodeResetResult{}, err
	}
	device.BindingCode = strings.TrimSpace(code)
	device.BindingCodeVersion++
	device.BindingCodeResetAt = s.now().UTC()
	if err := s.store.UpdateDevice(ctx, &device); err != nil {
		return CodeResetResult{}, err
	}
	return CodeResetResult{DeviceID: device.DeviceID, BindingCode: device.BindingCode}, nil
}

func (s *Service) ListMembers(ctx context.Context, adminAccountID uint) ([]MemberBinding, error) {
	_, device, err := s.adminBinding(ctx, adminAccountID)
	if err != nil {
		return nil, err
	}
	bindings, err := s.store.ListMembers(ctx, device.ID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberBinding, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, MemberBinding{
			BindingID: binding.ID,
			AccountID: binding.AccountID,
			Role:      binding.Role,
			BoundAt:   binding.BoundAt,
		})
	}
	return out, nil
}

func (s *Service) RemoveMember(ctx context.Context, adminAccountID, memberAccountID uint) error {
	_, device, err := s.adminBinding(ctx, adminAccountID)
	if err != nil {
		return err
	}
	member, err := s.store.GetMember(ctx, device.ID, memberAccountID)
	if err != nil {
		return err
	}
	return s.store.DeleteBinding(ctx, member.ID)
}

func (s *Service) adminBinding(ctx context.Context, accountID uint) (DeviceBinding, AnbanDevice, error) {
	if accountID == 0 {
		return DeviceBinding{}, AnbanDevice{}, ErrInvalidInput
	}
	binding, err := s.store.GetBindingByAccount(ctx, accountID)
	if err != nil {
		return DeviceBinding{}, AnbanDevice{}, err
	}
	if binding.Role != RoleAdmin {
		return DeviceBinding{}, AnbanDevice{}, ErrAdminRequired
	}
	device, err := s.store.GetDeviceByID(ctx, binding.AnbanDeviceID)
	if err != nil {
		return DeviceBinding{}, AnbanDevice{}, err
	}
	return binding, device, nil
}

func bindingView(binding DeviceBinding, device AnbanDevice) BindingView {
	return BindingView{
		BindingID:         binding.ID,
		DeviceRecordID:    device.ID,
		DeviceID:          device.DeviceID,
		DeviceDisplayName: device.DisplayName,
		ElderDisplayName:  device.ElderDisplayName,
		Role:              binding.Role,
	}
}

func validRole(role Role) bool {
	return role == RoleAdmin || role == RoleMember
}

func generateBindingCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ANBAN-%06d", n.Int64()), nil
}
