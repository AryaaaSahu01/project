package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	SessionLifetime = 7 * 24 * time.Hour
	PasswordCost    = 12
)

type Store interface {
	Register(ctx context.Context, email, passwordHash, displayName string, tokenHash [32]byte, expiresAt time.Time) (Account, error)
	PasswordRecord(ctx context.Context, email string) (userID, passwordHash string, err error)
	CreateSession(ctx context.Context, userID string, tokenHash [32]byte, expiresAt time.Time) error
	GetAccountByTokenHash(ctx context.Context, tokenHash [32]byte) (Account, error)
	UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (Profile, error)
	ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error)
	RevokeSession(ctx context.Context, tokenHash [32]byte) error
}

type Service struct {
	store Store
	now   func() time.Time
	// The precomputed hash minimizes user-enumeration timing differences.
	fakePasswordHash []byte
}

func NewService(store Store) (*Service, error) {
	fake, err := bcrypt.GenerateFromPassword([]byte("nonexistent_account_password"), PasswordCost)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, now: time.Now, fakePasswordHash: fake}, nil
}

func cleanEmail(input string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input))
	if len(email) < 3 || len(email) > 254 {
		return "", ErrInvalidInput
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || strings.ContainsAny(email, " \r\n\t") {
		return "", ErrInvalidInput
	}
	return email, nil
}

func validPassword(p string) bool {
	return len(p) >= 12 && len(p) <= 72
}

func validDisplayName(n string) bool {
	return n != "" && utf8.RuneCountInString(n) <= 100
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (Session, error) {
	email, err := cleanEmail(input.Email)
	if err != nil || !validPassword(input.Password) {
		return Session{}, ErrInvalidInput
	}
	name := strings.TrimSpace(input.DisplayName)
	if !validDisplayName(name) {
		return Session{}, ErrInvalidInput
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), PasswordCost)
	if err != nil {
		return Session{}, err
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return Session{}, err
	}
	expires := s.now().UTC().Add(SessionLifetime)
	account, err := s.store.Register(ctx, email, string(passwordHash), name, tokenHash, expires)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, TokenType: "Bearer", ExpiresAt: expires, Account: account}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	email, err := cleanEmail(input.Email)
	if err != nil || input.Password == "" || len(input.Password) > 72 {
		return Session{}, ErrInvalidCredential
	}
	userID, storedHash, err := s.store.PasswordRecord(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.fakePasswordHash, []byte(input.Password))
		return Session{}, ErrInvalidCredential
	}
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(input.Password)) != nil {
		return Session{}, ErrInvalidCredential
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return Session{}, err
	}
	expires := s.now().UTC().Add(SessionLifetime)
	if err := s.store.CreateSession(ctx, userID, tokenHash, expires); err != nil {
		return Session{}, err
	}
	account, err := s.store.GetAccountByTokenHash(ctx, tokenHash)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, TokenType: "Bearer", ExpiresAt: expires, Account: account}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Account, error) {
	if len(token) != 43 { // base64url representation of 32 random bytes
		return Account{}, ErrUnauthorized
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return Account{}, ErrUnauthorized
	}
	account, err := s.store.GetAccountByTokenHash(ctx, sha256.Sum256(decoded))
	if errors.Is(err, ErrNotFound) {
		return Account{}, ErrUnauthorized
	}
	return account, err
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (Profile, error) {
	if input.DisplayName == nil && input.Bio == nil && input.Timezone == nil {
		return Profile{}, ErrInvalidInput
	}
	if input.DisplayName != nil {
		v := strings.TrimSpace(*input.DisplayName)
		if !validDisplayName(v) {
			return Profile{}, ErrInvalidInput
		}
		input.DisplayName = &v
	}
	if input.Bio != nil {
		v := strings.TrimSpace(*input.Bio)
		if utf8.RuneCountInString(v) > 2000 {
			return Profile{}, ErrInvalidInput
		}
		input.Bio = &v
	}
	if input.Timezone != nil {
		v := strings.TrimSpace(*input.Timezone)
		if len(v) > 100 {
			return Profile{}, ErrInvalidInput
		}
		if _, err := time.LoadLocation(v); err != nil || v == "" {
			return Profile{}, ErrInvalidInput
		}
		input.Timezone = &v
	}
	return s.store.UpdateProfile(ctx, userID, input)
}

func (s *Service) Workspaces(ctx context.Context, userID string) ([]Workspace, error) {
	return s.store.ListWorkspaces(ctx, userID)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return ErrUnauthorized
	}
	return s.store.RevokeSession(ctx, sha256.Sum256(decoded))
}

func newToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), sha256.Sum256(raw[:]), nil
}
