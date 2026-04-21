package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// Querier is the minimal database interface consumed by this package.
// Declared on the consumer side per idiomatic Go; both *pgxpool.Pool and
// pgx.Tx satisfy it, enabling transactional wrapping if needed.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// User is the user identity record returned from the database.
// Game-state fields (trophies, gold, diamonds, energy) live in profiles.
type User struct {
	ID           uuid.UUID
	Email        string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser is the input required to create a new account.
type NewUser struct {
	Email        string
	Username     string
	PasswordHash string
}

// userStore wraps a connection pool with the auth-scoped data access methods.
type userStore struct {
	pool *pgxpool.Pool
}

func newUserStore(pool *pgxpool.Pool) *userStore {
	return &userStore{pool: pool}
}

// CreateUser inserts a new row into users and returns the created User.
// The UUID is generated application-side so we do not rely on gen_random_uuid().
// Maps unique-constraint violations to ErrEmailTaken or ErrUsernameTaken.
func (s *userStore) CreateUser(ctx context.Context, nu NewUser) (User, error) {
	id := uuid.New()
	const q = `
		INSERT INTO users (id, email, username, password_hash)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, email, username, password_hash, created_at, updated_at`

	var u User
	var idStr string
	err := s.pool.QueryRow(ctx, q, id.String(), nu.Email, nu.Username, nu.PasswordHash).Scan(
		&idStr, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isConstraintError(err, "uq_users_email") {
			return User{}, ErrEmailTaken
		}
		if isConstraintError(err, "uq_users_username") {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}

	parsed, err := uuid.Parse(idStr)
	if err != nil {
		return User{}, fmt.Errorf("parse returned user id %q: %w", idStr, err)
	}
	u.ID = parsed
	return u, nil
}

// GetUserByEmail looks up a user by their email address.
// Returns ErrNotFound when no row exists.
func (s *userStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `
		SELECT id::text, email, username, password_hash, created_at, updated_at
		FROM   users
		WHERE  email = $1`

	return s.scanUser(s.pool.QueryRow(ctx, q, email))
}

// GetUserByID looks up a user by their UUID primary key.
// Returns ErrNotFound when no row exists.
func (s *userStore) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	const q = `
		SELECT id::text, email, username, password_hash, created_at, updated_at
		FROM   users
		WHERE  id = $1::uuid`

	return s.scanUser(s.pool.QueryRow(ctx, q, id.String()))
}

// scanUser extracts a User from a single pgx.Row.
func (s *userStore) scanUser(row pgx.Row) (User, error) {
	var u User
	var idStr string
	err := row.Scan(&idStr, &u.Email, &u.Username, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return User{}, fmt.Errorf("parse user id %q: %w", idStr, err)
	}
	u.ID = id
	return u, nil
}

// isConstraintError reports whether err is a PostgreSQL unique_violation (23505)
// for the named constraint.
func isConstraintError(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraint
}
