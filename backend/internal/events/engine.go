// Package events implements the timed-event framework: active events are
// persisted in the DB, per-user progress is tracked across match results, and
// reward tiers can be claimed once the threshold is reached.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── domain types ──────────────────────────────────────────────────────────────

// Event is a timed challenge with configurable reward tiers.
type Event struct {
	ID          uuid.UUID
	Kind        string
	Name        string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	Config      json.RawMessage
}

// Progress tracks a single user's state within an event.
type Progress struct {
	EventID        uuid.UUID
	UserID         uuid.UUID
	Data           json.RawMessage // kind-specific progress object, e.g. {"count":42}
	ClaimedRewards []int           // sorted tier indexes already claimed
	UpdatedAt      time.Time
}

// Tier is a single reward threshold within an event.
type Tier struct {
	Threshold int64            `json:"threshold"`
	Rewards   map[string]int64 `json:"rewards"` // resource name → amount
}

// ── Kind interface ────────────────────────────────────────────────────────────

// Kind is the strategy interface every event type must implement. Each Kind
// handles progress accumulation and reward tier definition for its event type.
type Kind interface {
	// UpdateProgress merges a match result into existing progress data.
	// existing may be nil for the first update (treat as empty / zero state).
	UpdateProgress(existing json.RawMessage, monstersKilled, wavesCleared int, victory bool) (json.RawMessage, error)
	// ProgressValue extracts the comparable scalar (e.g. kill count) from
	// progress data. Used to check whether a reward tier threshold is met.
	ProgressValue(data json.RawMessage) (int64, error)
	// Tiers parses the event config and returns the ordered reward tiers.
	Tiers(config json.RawMessage) ([]Tier, error)
}

// ── Awarder interface ─────────────────────────────────────────────────────────

// Awarder credits in-game resources to a user when a reward tier is claimed.
// Declared consumer-side; implemented by a thin adapter over game.ResourceService
// in the router.
type Awarder interface {
	AddGold(ctx context.Context, userID uuid.UUID, amount int64) error
	AddDiamonds(ctx context.Context, userID uuid.UUID, amount int64) error
}

// ── Store interface ───────────────────────────────────────────────────────────

// Store abstracts event persistence. Declared consumer-side so tests can
// provide a fake without a real database.
type Store interface {
	// ActiveAndUpcoming returns events whose window overlaps [now, now+lookahead].
	// The lookahead allows the API to show events that start soon.
	ActiveAndUpcoming(ctx context.Context, now time.Time) ([]Event, error)
	// GetEvent returns the event for the given ID or ErrEventNotFound.
	GetEvent(ctx context.Context, id uuid.UUID) (Event, error)
	// GetOrCreateProgress returns the progress row for (eventID, userID),
	// creating a zero-value row if none exists yet.
	GetOrCreateProgress(ctx context.Context, eventID, userID uuid.UUID) (Progress, error)
	// SaveProgress upserts the full progress row.
	SaveProgress(ctx context.Context, p Progress) error
}

// ── Engine ────────────────────────────────────────────────────────────────────

// Engine is the top-level event service. It is safe for concurrent use.
type Engine struct {
	store   Store
	awarder Awarder
	kinds   map[string]Kind
	now     func() time.Time
}

// NewEngine constructs an Engine backed by pool and awarder. The built-in
// KillNMonsters kind is registered automatically.
func NewEngine(pool *pgxpool.Pool, awarder Awarder) *Engine {
	return &Engine{
		store:   newPgStore(pool),
		awarder: awarder,
		kinds:   defaultKinds(),
		now:     time.Now,
	}
}

// NewEngineWithStore constructs an Engine from explicit dependencies.
// Intended for tests.
func NewEngineWithStore(store Store, awarder Awarder, now func() time.Time) *Engine {
	return &Engine{
		store:   store,
		awarder: awarder,
		kinds:   defaultKinds(),
		now:     now,
	}
}

func defaultKinds() map[string]Kind {
	return map[string]Kind{
		"kill_n_monsters": KillNMonsters{},
	}
}

// lookahead is how far into the future the active/upcoming list reaches.
const lookahead = 7 * 24 * time.Hour

// ActiveAndUpcoming returns events that are currently active or start within
// the next seven days, ordered by starts_at ascending.
func (e *Engine) ActiveAndUpcoming(ctx context.Context) ([]Event, error) {
	events, err := e.store.ActiveAndUpcoming(ctx, e.now())
	if err != nil {
		return nil, fmt.Errorf("active and upcoming events: %w", err)
	}
	return events, nil
}

// RecordMatchResult implements game.EventRecorder. It accumulates progress for
// every currently-active event. Errors are logged and not propagated so that a
// failing event update never blocks the match result path.
func (e *Engine) RecordMatchResult(ctx context.Context, userID uuid.UUID, monstersKilled, wavesCleared int, victory bool) error {
	now := e.now()

	events, err := e.store.ActiveAndUpcoming(ctx, now)
	if err != nil {
		slog.WarnContext(ctx, "events: load active events failed", "err", err)
		return nil
	}

	for _, ev := range events {
		// Only update progress for events that are currently running.
		if now.Before(ev.StartsAt) || !now.Before(ev.EndsAt) {
			continue
		}

		kind, ok := e.kinds[ev.Kind]
		if !ok {
			continue
		}

		p, err := e.store.GetOrCreateProgress(ctx, ev.ID, userID)
		if err != nil {
			slog.WarnContext(ctx, "events: get progress failed", "event_id", ev.ID, "user_id", userID, "err", err)
			continue
		}

		newData, err := kind.UpdateProgress(p.Data, monstersKilled, wavesCleared, victory)
		if err != nil {
			slog.WarnContext(ctx, "events: update progress failed", "event_id", ev.ID, "kind", ev.Kind, "err", err)
			continue
		}

		p.Data = newData
		p.UpdatedAt = now
		if err := e.store.SaveProgress(ctx, p); err != nil {
			slog.WarnContext(ctx, "events: save progress failed", "event_id", ev.ID, "user_id", userID, "err", err)
		}
	}
	return nil
}

// ClaimReward claims reward tier tierIndex for userID within eventID. It
// returns the rewards map (resource → amount) on success.
func (e *Engine) ClaimReward(ctx context.Context, userID, eventID uuid.UUID, tierIndex int) (map[string]int64, error) {
	ev, err := e.store.GetEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("claim reward: %w", err)
	}

	now := e.now()
	if now.Before(ev.StartsAt) || !now.Before(ev.EndsAt) {
		return nil, ErrEventNotActive
	}

	kind, ok := e.kinds[ev.Kind]
	if !ok {
		return nil, ErrEventNotFound
	}

	tiers, err := kind.Tiers(ev.Config)
	if err != nil {
		return nil, fmt.Errorf("claim reward: parse tiers: %w", err)
	}
	if tierIndex < 0 || tierIndex >= len(tiers) {
		return nil, ErrTierInvalid
	}
	tier := tiers[tierIndex]

	p, err := e.store.GetOrCreateProgress(ctx, eventID, userID)
	if err != nil {
		return nil, fmt.Errorf("claim reward: get progress: %w", err)
	}

	if slices.Contains(p.ClaimedRewards, tierIndex) {
		return nil, ErrTierAlreadyClaimed
	}

	val, err := kind.ProgressValue(p.Data)
	if err != nil {
		return nil, fmt.Errorf("claim reward: read progress value: %w", err)
	}
	if val < tier.Threshold {
		return nil, ErrTierNotReached
	}

	// Award resources; a partial failure is logged but does not abort the claim.
	for resource, amount := range tier.Rewards {
		switch resource {
		case "gold":
			if aerr := e.awarder.AddGold(ctx, userID, amount); aerr != nil {
				slog.ErrorContext(ctx, "events: award gold failed", "err", aerr, "user_id", userID, "amount", amount)
			}
		case "diamonds":
			if aerr := e.awarder.AddDiamonds(ctx, userID, amount); aerr != nil {
				slog.ErrorContext(ctx, "events: award diamonds failed", "err", aerr, "user_id", userID, "amount", amount)
			}
		}
	}

	p.ClaimedRewards = append(p.ClaimedRewards, tierIndex)
	p.UpdatedAt = now
	if err := e.store.SaveProgress(ctx, p); err != nil {
		return nil, fmt.Errorf("claim reward: save progress: %w", err)
	}

	return tier.Rewards, nil
}

// ── pgStore ───────────────────────────────────────────────────────────────────

type pgStore struct {
	pool *pgxpool.Pool
}

func newPgStore(pool *pgxpool.Pool) *pgStore {
	return &pgStore{pool: pool}
}

const eventColumns = `
	id::text, kind, name, description, starts_at, ends_at, config`

func scanEvent(row pgx.Row) (Event, error) {
	var ev Event
	var idStr string
	err := row.Scan(&idStr, &ev.Kind, &ev.Name, &ev.Description, &ev.StartsAt, &ev.EndsAt, &ev.Config)
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("scan event: %w", err)
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return Event{}, fmt.Errorf("parse event id %q: %w", idStr, err)
	}
	ev.ID = id
	return ev, nil
}

// ActiveAndUpcoming returns events active now or starting within lookahead.
func (s *pgStore) ActiveAndUpcoming(ctx context.Context, now time.Time) ([]Event, error) {
	const q = `
		SELECT` + eventColumns + `
		FROM   events
		WHERE  starts_at < $2 AND ends_at > $1
		ORDER  BY starts_at ASC`

	rows, err := s.pool.Query(ctx, q, now, now.Add(lookahead))
	if err != nil {
		return nil, fmt.Errorf("active and upcoming events: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *pgStore) GetEvent(ctx context.Context, id uuid.UUID) (Event, error) {
	const q = `
		SELECT` + eventColumns + `
		FROM   events
		WHERE  id = $1::uuid`

	ev, err := scanEvent(s.pool.QueryRow(ctx, q, id.String()))
	if err != nil {
		return Event{}, fmt.Errorf("get event: %w", err)
	}
	return ev, nil
}

func (s *pgStore) GetOrCreateProgress(ctx context.Context, eventID, userID uuid.UUID) (Progress, error) {
	// INSERT … ON CONFLICT DO NOTHING, then SELECT guarantees a row exists.
	const ins = `
		INSERT INTO event_progress (event_id, user_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (event_id, user_id) DO NOTHING`

	if _, err := s.pool.Exec(ctx, ins, eventID.String(), userID.String()); err != nil {
		return Progress{}, fmt.Errorf("get or create progress (insert): %w", err)
	}

	const sel = `
		SELECT event_id::text, user_id::text, progress, claimed_rewards, updated_at
		FROM   event_progress
		WHERE  event_id = $1::uuid AND user_id = $2::uuid`

	row := s.pool.QueryRow(ctx, sel, eventID.String(), userID.String())
	return scanProgress(row)
}

func (s *pgStore) SaveProgress(ctx context.Context, p Progress) error {
	const q = `
		INSERT INTO event_progress (event_id, user_id, progress, claimed_rewards, updated_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		ON CONFLICT (event_id, user_id) DO UPDATE
		SET progress        = EXCLUDED.progress,
		    claimed_rewards = EXCLUDED.claimed_rewards,
		    updated_at      = EXCLUDED.updated_at`

	claimedJSON, err := json.Marshal(p.ClaimedRewards)
	if err != nil {
		return fmt.Errorf("save progress: marshal claimed rewards: %w", err)
	}
	if _, err := s.pool.Exec(ctx, q,
		p.EventID.String(), p.UserID.String(),
		p.Data, claimedJSON, p.UpdatedAt,
	); err != nil {
		return fmt.Errorf("save progress: %w", err)
	}
	return nil
}

func scanProgress(row pgx.Row) (Progress, error) {
	var p Progress
	var evStr, uStr string
	var claimedJSON json.RawMessage
	err := row.Scan(&evStr, &uStr, &p.Data, &claimedJSON, &p.UpdatedAt)
	if err != nil {
		return Progress{}, fmt.Errorf("scan progress: %w", err)
	}
	ev, err := uuid.Parse(evStr)
	if err != nil {
		return Progress{}, fmt.Errorf("parse event_id %q: %w", evStr, err)
	}
	p.EventID = ev

	u, err := uuid.Parse(uStr)
	if err != nil {
		return Progress{}, fmt.Errorf("parse user_id %q: %w", uStr, err)
	}
	p.UserID = u

	if err := json.Unmarshal(claimedJSON, &p.ClaimedRewards); err != nil {
		return Progress{}, fmt.Errorf("unmarshal claimed rewards: %w", err)
	}
	if p.ClaimedRewards == nil {
		p.ClaimedRewards = []int{}
	}
	return p, nil
}
