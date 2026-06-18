package devicebinding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
)

func newBindingService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	bindingStore := NewStore(st.DB)
	if err := bindingStore.AutoMigrate(); err != nil {
		t.Fatalf("migrate binding store: %v", err)
	}
	svc := NewService(bindingStore, Options{
		Now: func() time.Time { return time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC) },
		CodeGenerator: func() (string, error) {
			return "ANBAN-999999", nil
		},
	})
	if _, err := svc.EnsureDevice(context.Background(), DeviceSeed{
		DeviceID:         "dev-001",
		BindingCode:      "ANBAN-111111",
		DisplayName:      "客厅安伴",
		ElderDisplayName: "王阿姨",
	}); err != nil {
		t.Fatalf("EnsureDevice error = %v", err)
	}
	return svc
}

func TestBindAdminAndMembers(t *testing.T) {
	ctx := context.Background()
	svc := newBindingService(t)

	admin, err := svc.Bind(ctx, BindRequest{AccountID: 1, Role: RoleAdmin, BindingCode: "ANBAN-111111"})
	if err != nil {
		t.Fatalf("Bind admin error = %v", err)
	}
	if admin.DeviceID != "dev-001" || admin.Role != RoleAdmin || admin.ElderDisplayName != "王阿姨" {
		t.Fatalf("admin binding = %+v", admin)
	}

	if _, err := svc.Bind(ctx, BindRequest{AccountID: 2, Role: RoleAdmin, BindingCode: "ANBAN-111111"}); !errors.Is(err, ErrAdminAlreadyBound) {
		t.Fatalf("second admin error = %v, want ErrAdminAlreadyBound", err)
	}

	if _, err := svc.Bind(ctx, BindRequest{AccountID: 2, Role: RoleMember, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind member 2 error = %v", err)
	}
	if _, err := svc.Bind(ctx, BindRequest{AccountID: 3, Role: RoleMember, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind member 3 error = %v", err)
	}

	members, err := svc.ListMembers(ctx, 1)
	if err != nil {
		t.Fatalf("ListMembers error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members len = %d, want 2", len(members))
	}
}

func TestSingleAccountCannotBindTwice(t *testing.T) {
	ctx := context.Background()
	svc := newBindingService(t)

	if _, err := svc.Bind(ctx, BindRequest{AccountID: 1, Role: RoleMember, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("first bind error = %v", err)
	}
	if _, err := svc.Bind(ctx, BindRequest{AccountID: 1, Role: RoleMember, BindingCode: "ANBAN-111111"}); !errors.Is(err, ErrAccountAlreadyBound) {
		t.Fatalf("second bind error = %v, want ErrAccountAlreadyBound", err)
	}
}

func TestAdminCanResetCodeRemoveMemberAndUnbind(t *testing.T) {
	ctx := context.Background()
	svc := newBindingService(t)

	if _, err := svc.Bind(ctx, BindRequest{AccountID: 1, Role: RoleAdmin, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind admin error = %v", err)
	}
	if _, err := svc.Bind(ctx, BindRequest{AccountID: 2, Role: RoleMember, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind member error = %v", err)
	}

	reset, err := svc.ResetBindingCode(ctx, 1)
	if err != nil {
		t.Fatalf("ResetBindingCode error = %v", err)
	}
	if reset.BindingCode != "ANBAN-999999" {
		t.Fatalf("reset code = %q", reset.BindingCode)
	}

	if err := svc.RemoveMember(ctx, 1, 2); err != nil {
		t.Fatalf("RemoveMember error = %v", err)
	}
	members, err := svc.ListMembers(ctx, 1)
	if err != nil {
		t.Fatalf("ListMembers error = %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("members len after remove = %d, want 0", len(members))
	}

	if err := svc.UnbindAdmin(ctx, 1); err != nil {
		t.Fatalf("UnbindAdmin error = %v", err)
	}
	if _, err := svc.Bind(ctx, BindRequest{AccountID: 3, Role: RoleAdmin, BindingCode: "ANBAN-999999"}); err != nil {
		t.Fatalf("Bind new admin after unbind error = %v", err)
	}
}

func TestMemberCannotManageDevice(t *testing.T) {
	ctx := context.Background()
	svc := newBindingService(t)

	if _, err := svc.Bind(ctx, BindRequest{AccountID: 1, Role: RoleAdmin, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind admin error = %v", err)
	}
	if _, err := svc.Bind(ctx, BindRequest{AccountID: 2, Role: RoleMember, BindingCode: "ANBAN-111111"}); err != nil {
		t.Fatalf("Bind member error = %v", err)
	}

	if _, err := svc.ResetBindingCode(ctx, 2); !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("member reset error = %v, want ErrAdminRequired", err)
	}
	if err := svc.RemoveMember(ctx, 2, 1); !errors.Is(err, ErrAdminRequired) {
		t.Fatalf("member remove error = %v, want ErrAdminRequired", err)
	}
}
