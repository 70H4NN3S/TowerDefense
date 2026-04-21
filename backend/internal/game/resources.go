package game

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

// Energy regeneration constants. One energy point is restored every
// EnergyRegenInterval. EnergyMax is the cap; computed energy above it is
// clamped silently.
const (
	EnergyRegenInterval = 30 * time.Minute
	EnergyMax           = 5
)

// Profile is the player's game-state record, distinct from auth identity.
type Profile struct {
	UserID          uuid.UUID
	DisplayName     string
	AvatarID        int
	Trophies        int64
	Gold            int64
	Diamonds        int64
	Energy          int // stored energy at EnergyUpdatedAt
	EnergyUpdatedAt time.Time
	XP              int64
	Level           int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ComputedEnergy returns the current energy accounting for lazy regeneration.
// It does not mutate the profile; call FlushEnergy to persist the result.
func (p *Profile) ComputedEnergy(now time.Time) int {
	elapsed := now.Sub(p.EnergyUpdatedAt)
	regen := int(elapsed / EnergyRegenInterval)
	total := p.Energy + regen
	if total > EnergyMax {
		return EnergyMax
	}
	return total
}

// ProfileStore abstracts the data-access operations needed by ResourceService.
// Declared consumer-side so tests can supply a fake without a real database.
type ProfileStore interface {
	CreateProfile(ctx context.Context, userID uuid.UUID) (Profile, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (Profile, error)
	UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) (Profile, error)
	UpdateAvatarID(ctx context.Context, userID uuid.UUID, avatarID int) (Profile, error)
	// AddGold credits amount (must be > 0) to the profile's gold balance.
	AddGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	// SpendGold debits amount (must be > 0) atomically, returning
	// ErrInsufficientGold if the balance would go negative.
	SpendGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	// AddDiamonds credits amount (must be > 0) to the diamond balance.
	AddDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	// SpendDiamonds debits amount (must be > 0) atomically, returning
	// ErrInsufficientDiamonds if the balance would go negative.
	SpendDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	// FlushEnergy persists a new energy value and resets energy_updated_at.
	FlushEnergy(ctx context.Context, userID uuid.UUID, energy int, now time.Time) (Profile, error)
}

// ResourceService provides the business-logic layer for player resources.
type ResourceService struct {
	store ProfileStore
	now   func() time.Time
}

// NewResourceService constructs a ResourceService backed by pool.
func NewResourceService(pool *pgxpool.Pool) *ResourceService {
	return &ResourceService{store: newProfileStore(pool), now: time.Now}
}

// NewResourceServiceWithStore constructs a ResourceService backed by the
// provided store and clock. Intended for tests.
func NewResourceServiceWithStore(store ProfileStore, now func() time.Time) *ResourceService {
	return &ResourceService{store: store, now: now}
}

// CreateProfile inserts a fresh profile row for userID with default values.
func (s *ResourceService) CreateProfile(ctx context.Context, userID uuid.UUID) (Profile, error) {
	p, err := s.store.CreateProfile(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("create profile: %w", err)
	}
	return p, nil
}

// GetProfile returns the profile for userID with ComputedEnergy applied.
func (s *ResourceService) GetProfile(ctx context.Context, userID uuid.UUID) (Profile, error) {
	p, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// UpdateDisplayName sets the player's display name.
func (s *ResourceService) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (Profile, error) {
	p, err := s.store.UpdateDisplayName(ctx, userID, name)
	if err != nil {
		return Profile{}, fmt.Errorf("update display name: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// UpdateAvatarID sets the player's avatar selection.
func (s *ResourceService) UpdateAvatarID(ctx context.Context, userID uuid.UUID, avatarID int) (Profile, error) {
	p, err := s.store.UpdateAvatarID(ctx, userID, avatarID)
	if err != nil {
		return Profile{}, fmt.Errorf("update avatar id: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// AddGold credits amount gold to the player. amount must be positive.
func (s *ResourceService) AddGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if amount <= 0 {
		return Profile{}, fmt.Errorf("add gold: amount must be positive, got %d", amount)
	}
	p, err := s.store.AddGold(ctx, userID, amount)
	if err != nil {
		return Profile{}, fmt.Errorf("add gold: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// SpendGold debits amount gold from the player atomically.
// Returns ErrInsufficientGold if the balance is too low.
func (s *ResourceService) SpendGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if amount <= 0 {
		return Profile{}, fmt.Errorf("spend gold: amount must be positive, got %d", amount)
	}
	p, err := s.store.SpendGold(ctx, userID, amount)
	if err != nil {
		return Profile{}, fmt.Errorf("spend gold: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// AddDiamonds credits amount diamonds to the player. amount must be positive.
func (s *ResourceService) AddDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if amount <= 0 {
		return Profile{}, fmt.Errorf("add diamonds: amount must be positive, got %d", amount)
	}
	p, err := s.store.AddDiamonds(ctx, userID, amount)
	if err != nil {
		return Profile{}, fmt.Errorf("add diamonds: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// SpendDiamonds debits amount diamonds from the player atomically.
// Returns ErrInsufficientDiamonds if the balance is too low.
func (s *ResourceService) SpendDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if amount <= 0 {
		return Profile{}, fmt.Errorf("spend diamonds: amount must be positive, got %d", amount)
	}
	p, err := s.store.SpendDiamonds(ctx, userID, amount)
	if err != nil {
		return Profile{}, fmt.Errorf("spend diamonds: %w", err)
	}
	p.Energy = p.ComputedEnergy(s.now())
	return p, nil
}

// SpendEnergy debits amount energy from the player. The current energy is
// computed via lazy regeneration first. Returns ErrInsufficientEnergy if the
// computed balance is too low; on success, persists the new value and resets
// the regen clock.
func (s *ResourceService) SpendEnergy(ctx context.Context, userID uuid.UUID, amount int) (Profile, error) {
	if amount <= 0 {
		return Profile{}, fmt.Errorf("spend energy: amount must be positive, got %d", amount)
	}
	p, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("spend energy: %w", err)
	}
	now := s.now()
	current := p.ComputedEnergy(now)
	if current < amount {
		return Profile{}, ErrInsufficientEnergy
	}
	p, err = s.store.FlushEnergy(ctx, userID, current-amount, now)
	if err != nil {
		return Profile{}, fmt.Errorf("spend energy: %w", err)
	}
	p.Energy = current - amount
	return p, nil
}

// ── profileStore ─────────────────────────────────────────────────────────────

type profileStore struct {
	pool *pgxpool.Pool
}

func newProfileStore(pool *pgxpool.Pool) *profileStore {
	return &profileStore{pool: pool}
}

// profileColumns is the fixed SELECT / RETURNING column list for the profiles
// table. The order must match scanProfile exactly.
const profileColumns = `
	user_id::text, display_name, avatar_id,
	trophies, gold, diamonds, energy, energy_updated_at,
	xp, level, created_at, updated_at`

func scanProfile(row pgx.Row) (Profile, error) {
	var p Profile
	var idStr string
	err := row.Scan(
		&idStr, &p.DisplayName, &p.AvatarID,
		&p.Trophies, &p.Gold, &p.Diamonds, &p.Energy, &p.EnergyUpdatedAt,
		&p.XP, &p.Level, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("scan profile: %w", err)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Profile{}, fmt.Errorf("parse profile user_id %q: %w", idStr, err)
	}
	p.UserID = id
	return p, nil
}

func (s *profileStore) CreateProfile(ctx context.Context, userID uuid.UUID) (Profile, error) {
	// ON CONFLICT DO NOTHING makes this idempotent: calling CreateProfile for a
	// user that already has a profile is safe and returns the existing row.
	const q = `
		INSERT INTO profiles (user_id)
		VALUES ($1::uuid)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String()))
	if err != nil {
		// ON CONFLICT DO NOTHING returns no row; fall back to a SELECT.
		if errors.Is(err, ErrProfileNotFound) {
			return s.GetProfile(ctx, userID)
		}
		return Profile{}, fmt.Errorf("create profile: %w", err)
	}
	return p, nil
}

func (s *profileStore) GetProfile(ctx context.Context, userID uuid.UUID) (Profile, error) {
	const q = `
		SELECT` + profileColumns + `
		FROM   profiles
		WHERE  user_id = $1::uuid`

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String()))
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

func (s *profileStore) UpdateDisplayName(ctx context.Context, userID uuid.UUID, displayName string) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    display_name = $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), displayName))
	if err != nil {
		return Profile{}, fmt.Errorf("update display name: %w", err)
	}
	return p, nil
}

func (s *profileStore) UpdateAvatarID(ctx context.Context, userID uuid.UUID, avatarID int) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    avatar_id = $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), avatarID))
	if err != nil {
		return Profile{}, fmt.Errorf("update avatar id: %w", err)
	}
	return p, nil
}

func (s *profileStore) AddGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    gold = gold + $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), amount))
	if err != nil {
		return Profile{}, fmt.Errorf("add gold: %w", err)
	}
	return p, nil
}

func (s *profileStore) SpendGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	// The CHECK constraint ck_profiles_gold_nonneg prevents the balance from
	// going negative. A constraint violation is mapped to ErrInsufficientGold.
	const q = `
		UPDATE profiles
		SET    gold = gold - $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), amount))
	if err != nil {
		if isCheckViolation(err, "ck_profiles_gold_nonneg") {
			return Profile{}, ErrInsufficientGold
		}
		return Profile{}, fmt.Errorf("spend gold: %w", err)
	}
	return p, nil
}

func (s *profileStore) AddDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    diamonds = diamonds + $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), amount))
	if err != nil {
		return Profile{}, fmt.Errorf("add diamonds: %w", err)
	}
	return p, nil
}

func (s *profileStore) SpendDiamonds(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    diamonds = diamonds - $2, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), amount))
	if err != nil {
		if isCheckViolation(err, "ck_profiles_diamonds_nonneg") {
			return Profile{}, ErrInsufficientDiamonds
		}
		return Profile{}, fmt.Errorf("spend diamonds: %w", err)
	}
	return p, nil
}

func (s *profileStore) FlushEnergy(ctx context.Context, userID uuid.UUID, energy int, now time.Time) (Profile, error) {
	const q = `
		UPDATE profiles
		SET    energy = $2, energy_updated_at = $3, updated_at = now()
		WHERE  user_id = $1::uuid
		RETURNING` + profileColumns

	p, err := scanProfile(s.pool.QueryRow(ctx, q, userID.String(), energy, now))
	if err != nil {
		return Profile{}, fmt.Errorf("flush energy: %w", err)
	}
	return p, nil
}

// isCheckViolation reports whether err is a PostgreSQL check_violation (23514)
// for the named constraint.
func isCheckViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23514" &&
		pgErr.ConstraintName == constraint
}
