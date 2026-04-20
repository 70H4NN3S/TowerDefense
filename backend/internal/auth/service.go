package auth

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/johannesniedens/towerdefense/internal/models"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

const (
	accessTokenTTL  = time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour

	minPasswordLen = 12
	maxUsernameLen = 32
	maxEmailLen    = 254
)

// Store is the data-access interface consumed by Service.
// Declared consumer-side per idiomatic Go; *userStore satisfies it, as
// does any fake provided by internal/testutil/authtest for testing.
type Store interface {
	CreateUser(ctx context.Context, nu NewUser) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
}

// Service handles account registration, login, and token refresh.
type Service struct {
	store     Store
	jwtSecret []byte
}

// NewService constructs a Service backed by pool.
func NewService(pool *pgxpool.Pool, jwtSecret []byte) *Service {
	return &Service{
		store:     newUserStore(pool),
		jwtSecret: jwtSecret,
	}
}

// NewServiceWithStore constructs a Service backed by the provided store.
// Intended for tests that supply an in-memory store without a real database.
func NewServiceWithStore(store Store, jwtSecret []byte) *Service {
	return &Service{store: store, jwtSecret: jwtSecret}
}

// RegisterInput is the payload for account creation.
type RegisterInput struct {
	Email    string
	Username string
	Password string
}

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Register creates a new user account and returns a fresh token pair.
func (s *Service) Register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	if err := validateRegisterInput(in); err != nil {
		return TokenPair{}, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return TokenPair{}, fmt.Errorf("register: %w", err)
	}

	user, err := s.store.CreateUser(ctx, NewUser{
		Email:        strings.ToLower(strings.TrimSpace(in.Email)),
		Username:     strings.TrimSpace(in.Username),
		PasswordHash: hash,
	})
	if err != nil {
		return TokenPair{}, fmt.Errorf("register: %w", err)
	}

	return s.issueTokens(user.ID.String())
}

// LoginInput is the payload for a login attempt.
type LoginInput struct {
	Email    string
	Password string
}

// Login verifies credentials and returns a fresh token pair.
func (s *Service) Login(ctx context.Context, in LoginInput) (TokenPair, error) {
	user, err := s.store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		// Map ErrNotFound → ErrInvalidCredentials to avoid user-enumeration.
		if err == ErrNotFound {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, fmt.Errorf("login: %w", err)
	}

	if err := VerifyPassword(user.PasswordHash, in.Password); err != nil {
		return TokenPair{}, err // already ErrInvalidCredentials or wrapped
	}

	return s.issueTokens(user.ID.String())
}

// Refresh validates a refresh token and issues a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := ParseToken(refreshToken, s.jwtSecret)
	if err != nil {
		return TokenPair{}, err // ErrInvalidToken or ErrExpiredToken
	}

	// Validate that the subject is a well-formed UUID before use.
	id, err := uuid.Parse(claims.Sub)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}

	// Verify the user still exists.
	if _, err := s.store.GetUserByID(ctx, id); err != nil {
		return TokenPair{}, fmt.Errorf("refresh: %w", err)
	}

	return s.issueTokens(claims.Sub)
}

// issueTokens creates a new access + refresh token pair for a given subject.
func (s *Service) issueTokens(subject string) (TokenPair, error) {
	access, err := SignToken(subject, s.jwtSecret, accessTokenTTL)
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}
	refresh, err := SignToken(subject, s.jwtSecret, refreshTokenTTL)
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign refresh token: %w", err)
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// validateRegisterInput returns a *models.ValidationError if any field is invalid.
func validateRegisterInput(in RegisterInput) error {
	v := &models.ValidationError{}

	email := strings.TrimSpace(in.Email)
	if email == "" {
		v.Add("email", "required")
	} else if len(email) > maxEmailLen {
		v.Add("email", fmt.Sprintf("must not exceed %d characters", maxEmailLen))
	} else if !isValidEmailShape(email) {
		v.Add("email", "invalid format")
	}

	username := strings.TrimSpace(in.Username)
	if username == "" {
		v.Add("username", "required")
	} else if utf8.RuneCountInString(username) > maxUsernameLen {
		v.Add("username", fmt.Sprintf("must not exceed %d characters", maxUsernameLen))
	}

	if in.Password == "" {
		v.Add("password", "required")
	} else if utf8.RuneCountInString(in.Password) < minPasswordLen {
		v.Add("password", fmt.Sprintf("must be at least %d characters", minPasswordLen))
	} else if len(in.Password) > maxPasswordBytes {
		v.Add("password", fmt.Sprintf("must not exceed %d bytes", maxPasswordBytes))
	}

	if v.HasErrors() {
		return v
	}
	return nil
}

// isValidEmailShape does a conservative structural check on an email address.
// Full RFC 5321 compliance is not required; we just need to reject obvious garbage.
func isValidEmailShape(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) == 0 || len(domain) < 3 {
		return false
	}
	// Domain must contain at least one dot after the @.
	return strings.Contains(domain, ".")
}
