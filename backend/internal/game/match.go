package game

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/johannesniedens/towerdefense/internal/game/sim"
	"github.com/johannesniedens/towerdefense/internal/models"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// Match is the persisted record of a single game session.
type Match struct {
	ID        uuid.UUID
	PlayerOne uuid.UUID
	PlayerTwo *uuid.UUID // nil for single-player
	Mode      string
	MapID     string
	Seed      int64
	StartedAt time.Time
	EndedAt   *time.Time
	Winner    *uuid.UUID
	CreatedAt time.Time
}

// MatchSummary is submitted by the client when a match ends.
// The server performs a plausibility check before awarding prizes.
type MatchSummary struct {
	MonstersKilled int   `json:"monsters_killed"`
	WavesCleared   int   `json:"waves_cleared"`
	GateHP         int64 `json:"gate_hp"`
	Victory        bool  `json:"victory"`
	GoldEarned     int64 `json:"gold_earned"`
}

// MatchResult is returned after a successful SubmitResult call.
type MatchResult struct {
	Match       Match
	GoldAwarded int64
	TrophyDelta int64
}

// ── MatchStore ────────────────────────────────────────────────────────────────

// MatchStore abstracts match persistence. Declared consumer-side so tests can
// supply a fake.
type MatchStore interface {
	// InsertMatch persists a new match row and returns it.
	InsertMatch(ctx context.Context, m Match) (Match, error)
	// GetMatch returns the match for the given ID.
	GetMatch(ctx context.Context, id uuid.UUID) (Match, error)
	// EndMatch sets ended_at and winner for the given match.
	EndMatch(ctx context.Context, id uuid.UUID, winner *uuid.UUID, endedAt time.Time) (Match, error)
}

// ── MatchResourcer ────────────────────────────────────────────────────────────

// MatchResourcer is the subset of ResourceService consumed by MatchService.
type MatchResourcer interface {
	SpendEnergy(ctx context.Context, userID uuid.UUID, amount int) (Profile, error)
	AddGold(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
	AddTrophies(ctx context.Context, userID uuid.UUID, amount int64) (Profile, error)
}

// ── MatchService ──────────────────────────────────────────────────────────────

// MatchService implements single-player match lifecycle: starting a match and
// submitting its result.
type MatchService struct {
	store     MatchStore
	resources MatchResourcer
	now       func() time.Time
}

// NewMatchService constructs a MatchService backed by pool and resources.
func NewMatchService(pool *pgxpool.Pool, resources MatchResourcer) *MatchService {
	return &MatchService{
		store:     newMatchStore(pool),
		resources: resources,
		now:       time.Now,
	}
}

// NewMatchServiceWithStore constructs a MatchService from explicit
// dependencies. Intended for tests.
func NewMatchServiceWithStore(store MatchStore, resources MatchResourcer, now func() time.Time) *MatchService {
	return &MatchService{store: store, resources: resources, now: now}
}

// trophyRewardVictory is the trophy gain for winning a single-player match.
const trophyRewardVictory = 25

// StartSinglePlayer creates a new solo match for userID on mapID. It debits
// one energy from the player's profile and persists the match row.
func (s *MatchService) StartSinglePlayer(ctx context.Context, userID uuid.UUID, mapID string) (Match, error) {
	if _, _, ok := sim.LookupMap(mapID); !ok {
		return Match{}, ErrUnknownMap
	}

	_, err := s.resources.SpendEnergy(ctx, userID, 1)
	if err != nil {
		return Match{}, fmt.Errorf("start single player: %w", err)
	}

	seed, err := generateSeed()
	if err != nil {
		return Match{}, fmt.Errorf("start single player: %w", err)
	}

	m := Match{
		ID:        uuid.New(),
		PlayerOne: userID,
		Mode:      "solo",
		MapID:     mapID,
		Seed:      seed,
		StartedAt: s.now(),
		CreatedAt: s.now(),
	}
	m, err = s.store.InsertMatch(ctx, m)
	if err != nil {
		return Match{}, fmt.Errorf("start single player: %w", err)
	}
	return m, nil
}

// SubmitResult finalises a single-player match, validates the summary, and
// awards gold and trophies. It returns ErrMatchNotFound, ErrMatchNotOwned, or
// ErrMatchAlreadyEnded when those conditions hold. A models.ValidationError is
// returned when the summary fails the plausibility check.
func (s *MatchService) SubmitResult(ctx context.Context, userID, matchID uuid.UUID, summary MatchSummary) (MatchResult, error) {
	m, err := s.store.GetMatch(ctx, matchID)
	if err != nil {
		return MatchResult{}, fmt.Errorf("submit result: %w", err)
	}
	if m.PlayerOne != userID {
		return MatchResult{}, ErrMatchNotOwned
	}
	if m.EndedAt != nil {
		return MatchResult{}, ErrMatchAlreadyEnded
	}

	if err := validateSummary(summary, m.MapID); err != nil {
		return MatchResult{}, err
	}

	var goldAwarded, trophyDelta int64

	if summary.GoldEarned > 0 {
		if _, err := s.resources.AddGold(ctx, userID, summary.GoldEarned); err != nil {
			return MatchResult{}, fmt.Errorf("submit result: award gold: %w", err)
		}
		goldAwarded = summary.GoldEarned
	}

	if summary.Victory {
		if _, err := s.resources.AddTrophies(ctx, userID, trophyRewardVictory); err != nil {
			return MatchResult{}, fmt.Errorf("submit result: award trophies: %w", err)
		}
		trophyDelta = trophyRewardVictory
	}

	now := s.now()
	var winner *uuid.UUID
	if summary.Victory {
		winner = &userID
	}
	m, err = s.store.EndMatch(ctx, matchID, winner, now)
	if err != nil {
		return MatchResult{}, fmt.Errorf("submit result: %w", err)
	}

	return MatchResult{Match: m, GoldAwarded: goldAwarded, TrophyDelta: trophyDelta}, nil
}

// validateSummary checks that the submitted MatchSummary is plausible for the
// given map. It does not fully replay the simulation; it is a sanity guard
// against obviously cheated values.
func validateSummary(summary MatchSummary, mapID string) error {
	var ve models.ValidationError

	if summary.MonstersKilled < 0 {
		ve.Add("monsters_killed", "must be non-negative")
	}
	if summary.WavesCleared < 0 {
		ve.Add("waves_cleared", "must be non-negative")
	}
	if summary.GateHP < 0 {
		ve.Add("gate_hp", "must be non-negative")
	}
	if summary.GoldEarned < 0 {
		ve.Add("gold_earned", "must be non-negative")
	}

	_, waves, ok := sim.LookupMap(mapID)
	if !ok {
		ve.Add("map_id", "unknown map")
	} else {
		totalWaves := len(waves)
		if summary.WavesCleared > totalWaves {
			ve.Add("waves_cleared", "exceeds total waves for this map")
		}
		maxGold := sim.MaxGoldForMap(waves)
		if summary.GoldEarned > maxGold {
			ve.Add("gold_earned", "exceeds maximum possible for this map")
		}
	}

	if len(ve.Fields) > 0 {
		return &ve
	}
	return nil
}

// generateSeed returns a cryptographically random int64 for use as a match seed.
func generateSeed() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("generate seed: %w", err)
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

// ── matchStore ────────────────────────────────────────────────────────────────

type matchStore struct {
	pool *pgxpool.Pool
}

func newMatchStore(pool *pgxpool.Pool) *matchStore {
	return &matchStore{pool: pool}
}

const matchColumns = `
	id::text, player_one::text, player_two::text, mode, map_id,
	seed, started_at, ended_at, winner::text, created_at`

func scanMatch(row pgx.Row) (Match, error) {
	var m Match
	var idStr, p1Str string
	var p2Str, winnerStr *string
	err := row.Scan(
		&idStr, &p1Str, &p2Str, &m.Mode, &m.MapID,
		&m.Seed, &m.StartedAt, &m.EndedAt, &winnerStr, &m.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Match{}, ErrMatchNotFound
	}
	if err != nil {
		return Match{}, fmt.Errorf("scan match: %w", err)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return Match{}, fmt.Errorf("parse match id %q: %w", idStr, err)
	}
	m.ID = id

	p1, err := uuid.Parse(p1Str)
	if err != nil {
		return Match{}, fmt.Errorf("parse player_one %q: %w", p1Str, err)
	}
	m.PlayerOne = p1

	if p2Str != nil {
		p2, err := uuid.Parse(*p2Str)
		if err != nil {
			return Match{}, fmt.Errorf("parse player_two %q: %w", *p2Str, err)
		}
		m.PlayerTwo = &p2
	}

	if winnerStr != nil {
		w, err := uuid.Parse(*winnerStr)
		if err != nil {
			return Match{}, fmt.Errorf("parse winner %q: %w", *winnerStr, err)
		}
		m.Winner = &w
	}

	return m, nil
}

func (s *matchStore) InsertMatch(ctx context.Context, m Match) (Match, error) {
	const q = `
		INSERT INTO matches (id, player_one, player_two, mode, map_id, seed, started_at, created_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $7)
		RETURNING` + matchColumns

	row := s.pool.QueryRow(ctx, q,
		m.ID.String(), m.PlayerOne.String(), nil, m.Mode, m.MapID, m.Seed, m.StartedAt,
	)
	out, err := scanMatch(row)
	if err != nil {
		return Match{}, fmt.Errorf("insert match: %w", err)
	}
	return out, nil
}

func (s *matchStore) GetMatch(ctx context.Context, id uuid.UUID) (Match, error) {
	const q = `
		SELECT` + matchColumns + `
		FROM   matches
		WHERE  id = $1::uuid`

	m, err := scanMatch(s.pool.QueryRow(ctx, q, id.String()))
	if err != nil {
		return Match{}, fmt.Errorf("get match: %w", err)
	}
	return m, nil
}

func (s *matchStore) EndMatch(ctx context.Context, id uuid.UUID, winner *uuid.UUID, endedAt time.Time) (Match, error) {
	var winnerStr *string
	if winner != nil {
		s := winner.String()
		winnerStr = &s
	}

	const q = `
		UPDATE matches
		SET    ended_at = $2, winner = $3::uuid
		WHERE  id = $1::uuid
		RETURNING` + matchColumns

	m, err := scanMatch(s.pool.QueryRow(ctx, q, id.String(), endedAt, winnerStr))
	if err != nil {
		return Match{}, fmt.Errorf("end match: %w", err)
	}
	return m, nil
}
