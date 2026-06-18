package account

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
)

func newTestService(t *testing.T) (*Service, *Store) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	accountStore := NewStore(st.DB)
	if err := accountStore.AutoMigrate(); err != nil {
		t.Fatalf("migrate account store: %v", err)
	}
	svc := NewService(accountStore, Options{
		Now: func() time.Time { return time.Date(2026, 6, 18, 8, 0, 0, 0, time.UTC) },
		TokenGenerator: func() (string, error) {
			return "plain-token", nil
		},
		DevVerificationCode: "123456",
	})
	return svc, accountStore
}

func TestRegisterHashesPasswordAndReturnsBearerSession(t *testing.T) {
	ctx := context.Background()
	svc, st := newTestService(t)

	resp, err := svc.Register(ctx, RegisterRequest{
		Phone:    " 13800000000 ",
		Password: "secret123",
		Nickname: " 小兰 ",
	})
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}

	if resp.Token != "plain-token" {
		t.Fatalf("token = %q, want generated token", resp.Token)
	}
	if resp.Account.Phone != "138****0000" {
		t.Fatalf("masked phone = %q", resp.Account.Phone)
	}
	if resp.Account.Nickname != "小兰" {
		t.Fatalf("nickname = %q", resp.Account.Nickname)
	}

	got, err := st.GetByPhone(ctx, "13800000000")
	if err != nil {
		t.Fatalf("GetByPhone error = %v", err)
	}
	if got.PasswordHash == "" || strings.Contains(got.PasswordHash, "secret123") {
		t.Fatalf("password hash was not stored safely: %q", got.PasswordHash)
	}

	accountFromToken, err := svc.Authenticate(ctx, "Bearer plain-token")
	if err != nil {
		t.Fatalf("Authenticate error = %v", err)
	}
	if accountFromToken.ID != got.ID {
		t.Fatalf("authenticated account id = %d, want %d", accountFromToken.ID, got.ID)
	}
}

func TestDuplicatePhoneAndWrongPasswordFail(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	if _, err := svc.Register(ctx, RegisterRequest{Phone: "13800000000", Password: "secret123"}); err != nil {
		t.Fatalf("first Register error = %v", err)
	}
	if _, err := svc.Register(ctx, RegisterRequest{Phone: "13800000000", Password: "secret123"}); !errors.Is(err, ErrDuplicatePhone) {
		t.Fatalf("duplicate Register error = %v, want ErrDuplicatePhone", err)
	}
	if _, err := svc.Login(ctx, LoginRequest{Phone: "13800000000", Password: "bad-password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v, want ErrInvalidCredentials", err)
	}
}

func TestVerificationCodeLoginCreatesOrReusesAccount(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	code, err := svc.SendVerificationCode(ctx, VerificationCodeRequest{
		Phone:   "13800000001",
		Purpose: "login",
	})
	if err != nil {
		t.Fatalf("SendVerificationCode error = %v", err)
	}
	if code.DebugCode != "123456" {
		t.Fatalf("debug code = %q, want dev code", code.DebugCode)
	}

	resp, err := svc.CodeLogin(ctx, CodeLoginRequest{
		Phone: "13800000001",
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("CodeLogin error = %v", err)
	}
	if resp.Account.AccountID == 0 {
		t.Fatalf("account id should be set")
	}

	if _, err := svc.CodeLogin(ctx, CodeLoginRequest{Phone: "13800000001", Code: "123456"}); !errors.Is(err, ErrInvalidVerificationCode) {
		t.Fatalf("reused code error = %v, want ErrInvalidVerificationCode", err)
	}
}

func TestUpdateProfileAndLogout(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)
	resp, err := svc.Register(ctx, RegisterRequest{Phone: "13800000002", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}

	updated, err := svc.UpdateProfile(ctx, resp.Account.AccountID, UpdateProfileRequest{
		Nickname:            "小鑫",
		RealName:            "鑫鑫",
		RelationshipToElder: "儿子",
		AvatarColor:         "#E89A6A",
	})
	if err != nil {
		t.Fatalf("UpdateProfile error = %v", err)
	}
	if updated.DisplayName != "小鑫" || updated.RelationshipToElder != "儿子" {
		t.Fatalf("updated profile = %+v", updated)
	}

	if err := svc.Logout(ctx, resp.Token); err != nil {
		t.Fatalf("Logout error = %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+resp.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Authenticate after logout error = %v, want ErrUnauthorized", err)
	}
}
