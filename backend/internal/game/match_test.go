package game

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/models"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeMatchStore ────────────────────────────────────────────────────────────

type fakeMatchStore struct {
	mu          sync.Mutex
	matches     map[uuid.UUID]Match
	endMatchErr error // if non-nil, EndMatch returns this error
}

func newFakeMatchStore() *fakeMatchStore {
	return &fakeMatchStore{matches: make(map[uuid.UUID]Match)}
}

func (f *fakeMatchStore) InsertMatch(_ context.Context, m Match) (Match, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.matches[m.ID] = m
	return m, nil
}

func (f *fakeMatchStore) GetMatch(_ context.Context, id uuid.UUID) (Match, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.matches[id]
	if !ok {
		return Match{}, ErrMatchNotFound
	}
	return m, nil
}

func (f *fakeMatchStore) EndMatch(_ context.Context, id uuid.UUID, winner *uuid.UUID, endedAt time.Time) (Match, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.endMatchErr != nil {
		return Match{}, f.endMatchErr
	}
	m, ok := f.matches[id]
	if !ok {
		return Match{}, ErrMatchNotFound
	}
	m.EndedAt = &endedAt
	m.Winner = winner
	f.matches[id] = m
	return m, nil
}

// ── fakeMatchResourcer ────────────────────────────────────────────────────────

type fakeMatchResourcer struct {
	mu       sync.Mutex
	profiles map[uuid.UUID]Profile
	// optionally inject errors
	spendEnergyErr error
	addGoldErr     error
	addTrophiesErr error
}

func newFakeMatchResourcer(users ...uuid.UUID) *fakeMatchResourcer {
	r := &fakeMatchResourcer{profiles: make(map[uuid.UUID]Profile)}
	for _, u := range users {
		r.profiles[u] = Profile{UserID: u, Energy: EnergyMax, Gold: 0, Trophies: 100}
	}
	return r
}

func (f *fakeMatchResourcer) SpendEnergy(_ context.Context, userID uuid.UUID, amount int) (Profile, error) {
	if f.spendEnergyErr != nil {
		return Profile{}, f.spendEnergyErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	if p.Energy < amount {
		return Profile{}, ErrInsufficientEnergy
	}
	p.Energy -= amount
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeMatchResourcer) AddGold(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if f.addGoldErr != nil {
		return Profile{}, f.addGoldErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	p.Gold += amount
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeMatchResourcer) AddTrophies(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	if f.addTrophiesErr != nil {
		return Profile{}, f.addTrophiesErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	p.Trophies += amount
	f.profiles[userID] = p
	return p, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestMatchService(store *fakeMatchStore, res *fakeMatchResourcer) *MatchService {
	return NewMatchServiceWithStore(store, res, func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	})
}

// ── StartSinglePlayer ─────────────────────────────────────────────────────────

func TestMatchService_StartSinglePlayer_HappyPath(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(userID)
	svc := newTestMatchService(store, res)

	m, err := svc.StartSinglePlayer(context.Background(), userID, "alpha")
	if err != nil {
		t.Fatalf("StartSinglePlayer error: %v", err)
	}
	if m.PlayerOne != userID {
		t.Errorf("PlayerOne = %v, want %v", m.PlayerOne, userID)
	}
	if m.Mode != "solo" {
		t.Errorf("Mode = %q, want solo", m.Mode)
	}
	if m.MapID != "alpha" {
		t.Errorf("MapID = %q, want alpha", m.MapID)
	}
	if m.EndedAt != nil {
		t.Error("EndedAt should be nil for a new match")
	}
	// Verify energy was deducted.
	if res.profiles[userID].Energy != EnergyMax-1 {
		t.Errorf("energy = %d, want %d", res.profiles[userID].Energy, EnergyMax-1)
	}
	// Verify match is persisted.
	stored, err := store.GetMatch(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("GetMatch after insert: %v", err)
	}
	if stored.ID != m.ID {
		t.Errorf("stored match ID mismatch")
	}
}

func TestMatchService_StartSinglePlayer_UnknownMap(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc := newTestMatchService(newFakeMatchStore(), newFakeMatchResourcer(userID))
	_, err := svc.StartSinglePlayer(context.Background(), userID, "does-not-exist")
	if !errors.Is(err, ErrUnknownMap) {
		t.Errorf("err = %v, want ErrUnknownMap", err)
	}
}

func TestMatchService_StartSinglePlayer_InsufficientEnergy(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	res := newFakeMatchResourcer(userID)
	res.profiles[userID] = Profile{UserID: userID, Energy: 0}
	svc := newTestMatchService(newFakeMatchStore(), res)

	_, err := svc.StartSinglePlayer(context.Background(), userID, "alpha")
	if !errors.Is(err, ErrInsufficientEnergy) {
		t.Errorf("err = %v, want ErrInsufficientEnergy", err)
	}
}

// ── SubmitResult ──────────────────────────────────────────────────────────────

func TestMatchService_SubmitResult_Victory(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(userID)
	svc := newTestMatchService(store, res)

	m, _ := svc.StartSinglePlayer(context.Background(), userID, "alpha")

	summary := MatchSummary{
		MonstersKilled: 10,
		WavesCleared:   3,
		GateHP:         5,
		Victory:        true,
		GoldEarned:     200,
	}
	result, err := svc.SubmitResult(context.Background(), userID, m.ID, summary)
	if err != nil {
		t.Fatalf("SubmitResult error: %v", err)
	}
	if result.GoldAwarded != 200 {
		t.Errorf("GoldAwarded = %d, want 200", result.GoldAwarded)
	}
	if result.TrophyDelta != trophyRewardVictory {
		t.Errorf("TrophyDelta = %d, want %d", result.TrophyDelta, trophyRewardVictory)
	}
	if result.Match.EndedAt == nil {
		t.Error("Match.EndedAt is nil after SubmitResult")
	}
	if result.Match.Winner == nil || *result.Match.Winner != userID {
		t.Errorf("Match.Winner = %v, want %v", result.Match.Winner, userID)
	}
	// Verify resources were credited.
	if res.profiles[userID].Gold != 200 {
		t.Errorf("gold in profile = %d, want 200", res.profiles[userID].Gold)
	}
	if res.profiles[userID].Trophies != 100+trophyRewardVictory {
		t.Errorf("trophies = %d, want %d", res.profiles[userID].Trophies, 100+trophyRewardVictory)
	}
}

func TestMatchService_SubmitResult_Defeat(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(userID)
	svc := newTestMatchService(store, res)

	m, _ := svc.StartSinglePlayer(context.Background(), userID, "alpha")

	summary := MatchSummary{
		MonstersKilled: 5,
		WavesCleared:   1,
		GateHP:         0,
		Victory:        false,
		GoldEarned:     50,
	}
	result, err := svc.SubmitResult(context.Background(), userID, m.ID, summary)
	if err != nil {
		t.Fatalf("SubmitResult error: %v", err)
	}
	if result.TrophyDelta != 0 {
		t.Errorf("TrophyDelta = %d, want 0 for defeat", result.TrophyDelta)
	}
	if result.Match.Winner != nil {
		t.Errorf("Winner should be nil for defeat, got %v", result.Match.Winner)
	}
	// Gold still awarded on defeat.
	if result.GoldAwarded != 50 {
		t.Errorf("GoldAwarded = %d, want 50", result.GoldAwarded)
	}
}

func TestMatchService_SubmitResult_ZeroGold(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(userID)
	svc := newTestMatchService(store, res)

	m, _ := svc.StartSinglePlayer(context.Background(), userID, "alpha")

	summary := MatchSummary{Victory: false, GoldEarned: 0}
	result, err := svc.SubmitResult(context.Background(), userID, m.ID, summary)
	if err != nil {
		t.Fatalf("SubmitResult error: %v", err)
	}
	if result.GoldAwarded != 0 {
		t.Errorf("GoldAwarded = %d, want 0 for zero-gold defeat", result.GoldAwarded)
	}
}

func TestMatchService_SubmitResult_NotFound(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc := newTestMatchService(newFakeMatchStore(), newFakeMatchResourcer(userID))

	_, err := svc.SubmitResult(context.Background(), userID, uuid.New(), MatchSummary{})
	if !errors.Is(err, ErrMatchNotFound) {
		t.Errorf("err = %v, want ErrMatchNotFound", err)
	}
}

func TestMatchService_SubmitResult_NotOwned(t *testing.T) {
	t.Parallel()
	owner := uuid.New()
	other := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(owner, other)
	svc := newTestMatchService(store, res)

	m, _ := svc.StartSinglePlayer(context.Background(), owner, "alpha")

	_, err := svc.SubmitResult(context.Background(), other, m.ID, MatchSummary{})
	if !errors.Is(err, ErrMatchNotOwned) {
		t.Errorf("err = %v, want ErrMatchNotOwned", err)
	}
}

func TestMatchService_SubmitResult_AlreadyEnded(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	store := newFakeMatchStore()
	res := newFakeMatchResourcer(userID)
	svc := newTestMatchService(store, res)

	m, _ := svc.StartSinglePlayer(context.Background(), userID, "alpha")
	summary := MatchSummary{Victory: true, GoldEarned: 100, WavesCleared: 2}
	_, _ = svc.SubmitResult(context.Background(), userID, m.ID, summary)

	_, err := svc.SubmitResult(context.Background(), userID, m.ID, summary)
	if !errors.Is(err, ErrMatchAlreadyEnded) {
		t.Errorf("err = %v, want ErrMatchAlreadyEnded", err)
	}
}

// ── validateSummary ───────────────────────────────────────────────────────────

func TestValidateSummary_HappyPath(t *testing.T) {
	t.Parallel()
	err := validateSummary(MatchSummary{
		MonstersKilled: 10,
		WavesCleared:   5,
		GateHP:         3,
		Victory:        true,
		GoldEarned:     766, // max possible for alpha
	}, "alpha")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSummary_NegativeValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		summary MatchSummary
		field   string
	}{
		{"negative monsters_killed", MatchSummary{MonstersKilled: -1}, "monsters_killed"},
		{"negative waves_cleared", MatchSummary{WavesCleared: -1}, "waves_cleared"},
		{"negative gate_hp", MatchSummary{GateHP: -1}, "gate_hp"},
		{"negative gold_earned", MatchSummary{GoldEarned: -1}, "gold_earned"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateSummary(tt.summary, "alpha")
			var ve *models.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
			found := false
			for _, f := range ve.Fields {
				if f.Field == tt.field {
					found = true
				}
			}
			if !found {
				t.Errorf("field %q not in ValidationError fields: %+v", tt.field, ve.Fields)
			}
		})
	}
}

func TestValidateSummary_GoldExceedsMax(t *testing.T) {
	t.Parallel()
	err := validateSummary(MatchSummary{GoldEarned: 99999}, "alpha")
	var ve *models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	found := false
	for _, f := range ve.Fields {
		if f.Field == "gold_earned" {
			found = true
		}
	}
	if !found {
		t.Error("gold_earned field not flagged when exceeding max")
	}
}

func TestValidateSummary_WavesClearedExceedsTotal(t *testing.T) {
	t.Parallel()
	err := validateSummary(MatchSummary{WavesCleared: 999}, "alpha")
	var ve *models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	found := false
	for _, f := range ve.Fields {
		if f.Field == "waves_cleared" {
			found = true
		}
	}
	if !found {
		t.Error("waves_cleared field not flagged when exceeding total")
	}
}

func TestValidateSummary_UnknownMap(t *testing.T) {
	t.Parallel()
	err := validateSummary(MatchSummary{}, "does-not-exist")
	var ve *models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}
