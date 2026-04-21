// Package leaderboard implements the leaderboard service: reading ranked
// player and alliance standings from materialized views, and periodically
// refreshing those views in the background.
package leaderboard

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── domain types ──────────────────────────────────────────────────────────────

// GlobalEntry is one row of the global_leaderboard materialized view.
type GlobalEntry struct {
	Rank     int64
	UserID   uuid.UUID
	Trophies int64
}

// AllianceEntry is one row of the alliance_leaderboard materialized view.
type AllianceEntry struct {
	AllianceID   uuid.UUID
	AllianceName string
	AllianceTag  string
	TotalTrophies int64
	MemberCount  int64
}

// MemberEntry is one member of an alliance ranked by their trophy count.
type MemberEntry struct {
	Rank     int64
	UserID   uuid.UUID
	Role     string
	Trophies int64
}

// ── store interface ───────────────────────────────────────────────────────────

// Store abstracts leaderboard reads and view refresh.
// Declared consumer-side so tests can supply fakes.
type Store interface {
	// GlobalPage returns up to limit global_leaderboard rows where rank >
	// afterRank, ordered by rank ASC.
	GlobalPage(ctx context.Context, afterRank int64, limit int) ([]GlobalEntry, error)
	// AlliancePage returns up to limit alliance_leaderboard rows where
	// total_trophies < afterTrophies (or all rows for the first page when
	// afterTrophies == -1), ordered by total_trophies DESC, alliance_id ASC.
	AlliancePage(ctx context.Context, afterTrophies int64, afterAllianceID uuid.UUID, limit int, firstPage bool) ([]AllianceEntry, error)
	// AllianceMembers returns all members of an alliance ranked by trophies DESC.
	// This hits the live profiles table, not a materialized view.
	AllianceMembers(ctx context.Context, allianceID uuid.UUID) ([]MemberEntry, error)
	// RefreshGlobal re-materialises global_leaderboard.
	RefreshGlobal(ctx context.Context) error
	// RefreshAlliance re-materialises alliance_leaderboard.
	RefreshAlliance(ctx context.Context) error
}

// ── service ───────────────────────────────────────────────────────────────────

// Service reads leaderboard data and manages background refresh.
type Service struct {
	store Store
}

// NewService constructs a Service backed by pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{store: newPGStore(pool)}
}

// NewServiceWithStore constructs a Service from an explicit Store.
// Intended for tests.
func NewServiceWithStore(store Store) *Service {
	return &Service{store: store}
}

// GlobalLeaderboard returns up to limit entries from the global leaderboard.
// afterRank is an exclusive rank cursor; pass 0 for the first page.
// limit is clamped to [1, 100].
func (s *Service) GlobalLeaderboard(ctx context.Context, afterRank int64, limit int) ([]GlobalEntry, error) {
	limit = clampLimit(limit)
	entries, err := s.store.GlobalPage(ctx, afterRank, limit)
	if err != nil {
		return nil, fmt.Errorf("global leaderboard: %w", err)
	}
	return entries, nil
}

// AllianceLeaderboard returns up to limit entries from the alliance leaderboard.
// For the first page, pass afterTrophies=-1. For subsequent pages pass the
// total_trophies of the last entry together with its alliance_id as a
// composite cursor.
// limit is clamped to [1, 100].
func (s *Service) AllianceLeaderboard(ctx context.Context, afterTrophies int64, afterAllianceID uuid.UUID, limit int, firstPage bool) ([]AllianceEntry, error) {
	limit = clampLimit(limit)
	entries, err := s.store.AlliancePage(ctx, afterTrophies, afterAllianceID, limit, firstPage)
	if err != nil {
		return nil, fmt.Errorf("alliance leaderboard: %w", err)
	}
	return entries, nil
}

// AllianceMemberLeaderboard returns all members of an alliance ranked by
// trophies descending. This always reads live data.
func (s *Service) AllianceMemberLeaderboard(ctx context.Context, allianceID uuid.UUID) ([]MemberEntry, error) {
	members, err := s.store.AllianceMembers(ctx, allianceID)
	if err != nil {
		return nil, fmt.Errorf("alliance member leaderboard: %w", err)
	}
	return members, nil
}

// StartRefresher starts a background goroutine that refreshes both materialized
// views every interval. It stops when ctx is cancelled.
// Refresh errors are logged and do not stop the loop.
func (s *Service) StartRefresher(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.store.RefreshGlobal(ctx); err != nil {
					slog.Error("leaderboard: refresh global", "err", err)
				}
				if err := s.store.RefreshAlliance(ctx); err != nil {
					slog.Error("leaderboard: refresh alliance", "err", err)
				}
			}
		}
	}()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func clampLimit(limit int) int {
	if limit <= 0 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// ── DB store ──────────────────────────────────────────────────────────────────

type pgStore struct {
	pool *pgxpool.Pool
}

func newPGStore(pool *pgxpool.Pool) *pgStore {
	return &pgStore{pool: pool}
}

func (s *pgStore) GlobalPage(ctx context.Context, afterRank int64, limit int) ([]GlobalEntry, error) {
	const q = `
		SELECT rank, user_id::text, trophies
		FROM   global_leaderboard
		WHERE  rank > $1
		ORDER BY rank ASC
		LIMIT  $2`

	rows, err := s.pool.Query(ctx, q, afterRank, limit)
	if err != nil {
		return nil, fmt.Errorf("global page: %w", err)
	}
	defer rows.Close()

	var out []GlobalEntry
	for rows.Next() {
		var e GlobalEntry
		var uStr string
		if err := rows.Scan(&e.Rank, &uStr, &e.Trophies); err != nil {
			return nil, fmt.Errorf("scan global entry: %w", err)
		}
		e.UserID, err = uuid.Parse(uStr)
		if err != nil {
			return nil, fmt.Errorf("parse global entry user_id %q: %w", uStr, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *pgStore) AlliancePage(ctx context.Context, afterTrophies int64, afterAllianceID uuid.UUID, limit int, firstPage bool) ([]AllianceEntry, error) {
	// Composite cursor: (total_trophies DESC, alliance_id ASC).
	// On the first page we skip the WHERE clause entirely.
	const qFirst = `
		SELECT alliance_id::text, alliance_name, alliance_tag, total_trophies, member_count
		FROM   alliance_leaderboard
		ORDER BY total_trophies DESC, alliance_id ASC
		LIMIT  $1`

	const qPage = `
		SELECT alliance_id::text, alliance_name, alliance_tag, total_trophies, member_count
		FROM   alliance_leaderboard
		WHERE  (total_trophies, alliance_id::text) < ($2, $3)
		ORDER BY total_trophies DESC, alliance_id ASC
		LIMIT  $1`

	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
		Err() error
	}

	if firstPage {
		r, err := s.pool.Query(ctx, qFirst, limit)
		if err != nil {
			return nil, fmt.Errorf("alliance page (first): %w", err)
		}
		rows = r
	} else {
		r, err := s.pool.Query(ctx, qPage, limit, afterTrophies, afterAllianceID.String())
		if err != nil {
			return nil, fmt.Errorf("alliance page: %w", err)
		}
		rows = r
	}
	defer rows.Close()

	var out []AllianceEntry
	for rows.Next() {
		var e AllianceEntry
		var aStr string
		if err := rows.Scan(&aStr, &e.AllianceName, &e.AllianceTag, &e.TotalTrophies, &e.MemberCount); err != nil {
			return nil, fmt.Errorf("scan alliance entry: %w", err)
		}
		var err error
		e.AllianceID, err = uuid.Parse(aStr)
		if err != nil {
			return nil, fmt.Errorf("parse alliance entry id %q: %w", aStr, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *pgStore) AllianceMembers(ctx context.Context, allianceID uuid.UUID) ([]MemberEntry, error) {
	const q = `
		SELECT
		    RANK() OVER (ORDER BY p.trophies DESC, am.user_id ASC) AS rank,
		    am.user_id::text,
		    am.role,
		    p.trophies
		FROM  alliance_members am
		JOIN  profiles p ON p.user_id = am.user_id
		WHERE am.alliance_id = $1::uuid
		ORDER BY rank ASC`

	rows, err := s.pool.Query(ctx, q, allianceID.String())
	if err != nil {
		return nil, fmt.Errorf("alliance members: %w", err)
	}
	defer rows.Close()

	var out []MemberEntry
	for rows.Next() {
		var e MemberEntry
		var uStr string
		if err := rows.Scan(&e.Rank, &uStr, &e.Role, &e.Trophies); err != nil {
			return nil, fmt.Errorf("scan member entry: %w", err)
		}
		e.UserID, err = uuid.Parse(uStr)
		if err != nil {
			return nil, fmt.Errorf("parse member entry user_id %q: %w", uStr, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *pgStore) RefreshGlobal(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY global_leaderboard`)
	if err != nil {
		return fmt.Errorf("refresh global_leaderboard: %w", err)
	}
	return nil
}

func (s *pgStore) RefreshAlliance(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY alliance_leaderboard`)
	if err != nil {
		return fmt.Errorf("refresh alliance_leaderboard: %w", err)
	}
	return nil
}
