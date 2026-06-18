package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSessionTTL      = 30 * 24 * time.Hour
	defaultVerificationTTL = 10 * time.Minute
)

type Options struct {
	Now                 func() time.Time
	TokenGenerator      func() (string, error)
	DevVerificationCode string
}

type Service struct {
	store               *Store
	now                 func() time.Time
	tokenGenerator      func() (string, error)
	devVerificationCode string
}

func NewService(store *Store, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	tokenGenerator := opts.TokenGenerator
	if tokenGenerator == nil {
		tokenGenerator = generateToken
	}
	devCode := strings.TrimSpace(opts.DevVerificationCode)
	if devCode == "" {
		devCode = "123456"
	}
	return &Service{
		store:               store,
		now:                 now,
		tokenGenerator:      tokenGenerator,
		devVerificationCode: devCode,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (AuthResponse, error) {
	phone := normalizePhone(req.Phone)
	password := strings.TrimSpace(req.Password)
	if phone == "" || len(password) < 6 {
		return AuthResponse{}, ErrInvalidInput
	}
	if _, err := s.store.GetByPhone(ctx, phone); err == nil {
		return AuthResponse{}, ErrDuplicatePhone
	} else if err != nil && err != ErrNotFound {
		return AuthResponse{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return AuthResponse{}, err
	}
	account := Account{
		Phone:        phone,
		PasswordHash: passwordHash,
		Nickname:     strings.TrimSpace(req.Nickname),
		Status:       AccountStatusActive,
	}
	if err := s.store.CreateAccount(ctx, &account); err != nil {
		if isUniqueConstraintError(err) {
			return AuthResponse{}, ErrDuplicatePhone
		}
		return AuthResponse{}, err
	}
	return s.createAuthResponse(ctx, account)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (AuthResponse, error) {
	phone := normalizePhone(req.Phone)
	password := strings.TrimSpace(req.Password)
	if phone == "" || password == "" {
		return AuthResponse{}, ErrInvalidInput
	}
	account, err := s.store.GetByPhone(ctx, phone)
	if err != nil {
		if err == ErrNotFound {
			return AuthResponse{}, ErrInvalidCredentials
		}
		return AuthResponse{}, err
	}
	if account.PasswordHash == "" || !verifyPassword(account.PasswordHash, password) {
		return AuthResponse{}, ErrInvalidCredentials
	}
	return s.createAuthResponse(ctx, account)
}

func (s *Service) SendVerificationCode(ctx context.Context, req VerificationCodeRequest) (VerificationCodeResponse, error) {
	phone := normalizePhone(req.Phone)
	purpose := normalizePurpose(req.Purpose)
	if phone == "" {
		return VerificationCodeResponse{}, ErrInvalidInput
	}
	code := s.devVerificationCode
	now := s.now().UTC()
	record := VerificationCode{
		Phone:     phone,
		CodeHash:  hashCode(code),
		Purpose:   purpose,
		ExpiresAt: now.Add(defaultVerificationTTL),
		CreatedAt: now,
	}
	if err := s.store.CreateVerificationCode(ctx, &record); err != nil {
		return VerificationCodeResponse{}, err
	}
	return VerificationCodeResponse{Sent: true, DebugCode: code}, nil
}

func (s *Service) CodeLogin(ctx context.Context, req CodeLoginRequest) (AuthResponse, error) {
	phone := normalizePhone(req.Phone)
	code := strings.TrimSpace(req.Code)
	if phone == "" || code == "" {
		return AuthResponse{}, ErrInvalidInput
	}
	record, err := s.store.LatestVerificationCode(ctx, phone, "login")
	if err != nil {
		return AuthResponse{}, err
	}
	now := s.now().UTC()
	if record.ExpiresAt.Before(now) || !compareHash(record.CodeHash, hashCode(code)) {
		return AuthResponse{}, ErrInvalidVerificationCode
	}
	if err := s.store.ConsumeVerificationCode(ctx, record.ID, now); err != nil {
		return AuthResponse{}, err
	}

	account, err := s.store.GetByPhone(ctx, phone)
	if err == ErrNotFound {
		account = Account{Phone: phone, Status: AccountStatusActive}
		if err := s.store.CreateAccount(ctx, &account); err != nil {
			return AuthResponse{}, err
		}
	} else if err != nil {
		return AuthResponse{}, err
	}
	return s.createAuthResponse(ctx, account)
}

func (s *Service) Authenticate(ctx context.Context, authorizationHeader string) (Account, error) {
	token := bearerToken(authorizationHeader)
	if token == "" {
		return Account{}, ErrUnauthorized
	}
	session, err := s.store.GetSessionByTokenHash(ctx, hashToken(token))
	if err != nil {
		return Account{}, err
	}
	if !session.ExpiresAt.After(s.now().UTC()) {
		return Account{}, ErrUnauthorized
	}
	return s.store.GetByID(ctx, session.AccountID)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return ErrUnauthorized
	}
	now := s.now().UTC()
	return s.store.RevokeSession(ctx, hashToken(token), now)
}

func (s *Service) GetAccount(ctx context.Context, accountID uint) (PublicAccount, error) {
	account, err := s.store.GetByID(ctx, accountID)
	if err != nil {
		return PublicAccount{}, err
	}
	return Public(account), nil
}

func (s *Service) UpdateProfile(ctx context.Context, accountID uint, req UpdateProfileRequest) (PublicAccount, error) {
	if accountID == 0 {
		return PublicAccount{}, ErrInvalidInput
	}
	account, err := s.store.GetByID(ctx, accountID)
	if err != nil {
		return PublicAccount{}, err
	}
	account.Nickname = strings.TrimSpace(req.Nickname)
	account.RealName = strings.TrimSpace(req.RealName)
	account.RelationshipToElder = strings.TrimSpace(req.RelationshipToElder)
	account.AvatarURL = strings.TrimSpace(req.AvatarURL)
	account.AvatarColor = strings.TrimSpace(req.AvatarColor)
	if err := s.store.UpdateAccount(ctx, &account); err != nil {
		return PublicAccount{}, err
	}
	return Public(account), nil
}

func (s *Service) createAuthResponse(ctx context.Context, account Account) (AuthResponse, error) {
	token, err := s.tokenGenerator()
	if err != nil {
		return AuthResponse{}, err
	}
	now := s.now().UTC()
	session := AuthSession{
		AccountID: account.ID,
		TokenHash: hashToken(token),
		ExpiresAt: now.Add(defaultSessionTTL),
		CreatedAt: now,
	}
	if err := s.store.CreateSession(ctx, &session); err != nil {
		return AuthResponse{}, err
	}
	return AuthResponse{Token: token, Account: Public(account)}, nil
}

func Public(account Account) PublicAccount {
	displayName := DisplayName(account)
	return PublicAccount{
		AccountID:           account.ID,
		Phone:               MaskPhone(account.Phone),
		Nickname:            account.Nickname,
		RealName:            account.RealName,
		RelationshipToElder: account.RelationshipToElder,
		AvatarURL:           account.AvatarURL,
		AvatarColor:         account.AvatarColor,
		DisplayName:         displayName,
	}
}

func DisplayName(account Account) string {
	if name := strings.TrimSpace(account.Nickname); name != "" {
		return name
	}
	return MaskPhone(account.Phone)
}

func MaskPhone(phone string) string {
	phone = normalizePhone(phone)
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func normalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

func normalizePurpose(purpose string) string {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return "login"
	}
	return purpose
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func generateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func compareHash(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func hashPassword(password string) (string, error) {
	value, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(value), err
}

func verifyPassword(stored, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
