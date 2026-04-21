package game

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeProfileStore ──────────────────────────────────────────────────────────

// fakeProfileStore is an in-memory implementation of ProfileStore for tests.
// It is safe for concurrent use; the mutex serialises all mutations.
type fakeProfileStore struct {
	mu       sync.Mutex
	profiles map[uuid.UUID]Profile
}

func newFakeStore() *fakeProfileStore {
	return &fakeProfileStore{profiles: make(map[uuid.UUID]Profile)}
}

func (f *fakeProfileStore) seedProfile(p Profile) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profiles[p.UserID] = p
}

func (f *fakeProfileStore) CreateProfile(_ context.Context, userID uuid.UUID) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := Profile{
		UserID:          userID,
		Energy:          EnergyMax,
		EnergyUpdatedAt: time.Now(),
		Level:           1,
	}
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileStore) GetProfile(_ context.Context, userID uuid.UUID) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	return p, nil
}

func (f *fakeProfileStore) UpdateProfile(_ context.Context, userID uuid.UUID, displayName *string, avatarID *int) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	if displayName != nil {
		p.DisplayName = *displayName
	}
	if avatarID != nil {
		p.AvatarID = *avatarID
	}
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileStore) AddGold(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
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

func (f *fakeProfileStore) SpendGold(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	if p.Gold < amount {
		return Profile{}, ErrInsufficientGold
	}
	p.Gold -= amount
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileStore) AddDiamonds(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	p.Diamonds += amount
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileStore) SpendDiamonds(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	if p.Diamonds < amount {
		return Profile{}, ErrInsufficientDiamonds
	}
	p.Diamonds -= amount
	f.profiles[userID] = p
	return p, nil
}

func (f *fakeProfileStore) AddTrophies(_ context.Context, userID uuid.UUID, amount int64) (Profile, error) {
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

func (f *fakeProfileStore) FlushEnergy(_ context.Context, userID uuid.UUID, energy int, now time.Time) (Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[userID]
	if !ok {
		return Profile{}, ErrProfileNotFound
	}
	p.Energy = energy
	p.EnergyUpdatedAt = now
	f.profiles[userID] = p
	return p, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestService(store *fakeProfileStore, now func() time.Time) *ResourceService {
	return NewResourceServiceWithStore(store, now)
}

func newSeededService(t *testing.T, gold, diamonds int64, energy int, energyAge time.Duration) (*ResourceService, uuid.UUID) {
	t.Helper()
	store := newFakeStore()
	id := uuid.New()
	store.seedProfile(Profile{
		UserID:          id,
		Gold:            gold,
		Diamonds:        diamonds,
		Energy:          energy,
		EnergyUpdatedAt: time.Now().Add(-energyAge),
		Level:           1,
	})
	svc := newTestService(store, time.Now)
	return svc, id
}

// ── ComputedEnergy ────────────────────────────────────────────────────────────

func TestComputedEnergy(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		stored int
		age    time.Duration
		want   int
	}{
		{"no regen when full", EnergyMax, 0, EnergyMax},
		{"no regen when zero elapsed", 2, 0, 2},
		{"one regen tick", 2, EnergyRegenInterval, 3},
		{"two regen ticks", 1, 2 * EnergyRegenInterval, 3},
		{"clamp to max", 4, 2 * EnergyRegenInterval, EnergyMax},
		{"full recovery from zero", 0, time.Duration(EnergyMax) * EnergyRegenInterval, EnergyMax},
		{"beyond max does not exceed cap", 0, 10 * EnergyRegenInterval, EnergyMax},
		{"partial interval is truncated", 2, EnergyRegenInterval - time.Second, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{Energy: tt.stored, EnergyUpdatedAt: base}
			got := p.ComputedEnergy(base.Add(tt.age))
			if got != tt.want {
				t.Errorf("ComputedEnergy = %d, want %d", got, tt.want)
			}
		})
	}
}

// ── AddGold / SpendGold ───────────────────────────────────────────────────────

func TestAddGold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		startGold   int64
		amount      int64
		wantGold    int64
		wantErr     bool
		errContains string
	}{
		{"add to zero", 0, 100, 100, false, ""},
		{"add to existing", 50, 25, 75, false, ""},
		{"zero amount rejected", 0, 0, 0, true, "positive"},
		{"negative amount rejected", 0, -1, 0, true, "positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, id := newSeededService(t, tt.startGold, 0, EnergyMax, 0)

			p, err := svc.AddGold(context.Background(), id, tt.amount)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Gold != tt.wantGold {
				t.Errorf("Gold = %d, want %d", p.Gold, tt.wantGold)
			}
		})
	}
}

func TestSpendGold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		startGold int64
		amount    int64
		wantGold  int64
		wantErr   error
	}{
		{"exact balance", 100, 100, 0, nil},
		{"partial spend", 200, 50, 150, nil},
		{"insufficient", 30, 50, 0, ErrInsufficientGold},
		{"zero amount", 100, 0, 0, nil}, // caught by service guard
		{"negative amount", 100, -5, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, id := newSeededService(t, tt.startGold, 0, EnergyMax, 0)

			p, err := svc.SpendGold(context.Background(), id, tt.amount)
			if tt.amount <= 0 {
				// Service must reject non-positive amounts before calling store.
				if err == nil {
					t.Fatal("expected error for non-positive amount, got nil")
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Gold != tt.wantGold {
				t.Errorf("Gold = %d, want %d", p.Gold, tt.wantGold)
			}
		})
	}
}

// ── AddDiamonds / SpendDiamonds ───────────────────────────────────────────────

func TestAddDiamonds(t *testing.T) {
	t.Parallel()
	svc, id := newSeededService(t, 0, 0, EnergyMax, 0)

	p, err := svc.AddDiamonds(context.Background(), id, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Diamonds != 500 {
		t.Errorf("Diamonds = %d, want 500", p.Diamonds)
	}
}

func TestSpendDiamonds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		startDiamonds int64
		amount        int64
		wantDiamonds  int64
		wantErr       error
	}{
		{"sufficient", 100, 30, 70, nil},
		{"exact balance", 50, 50, 0, nil},
		{"insufficient", 10, 20, 0, ErrInsufficientDiamonds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, id := newSeededService(t, 0, tt.startDiamonds, EnergyMax, 0)

			p, err := svc.SpendDiamonds(context.Background(), id, tt.amount)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Diamonds != tt.wantDiamonds {
				t.Errorf("Diamonds = %d, want %d", p.Diamonds, tt.wantDiamonds)
			}
		})
	}
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

func TestUpdateProfile(t *testing.T) {
	t.Parallel()

	name := func(s string) *string { return &s }
	avatarID := func(n int) *int { return &n }

	tests := []struct {
		name        string
		displayName *string
		avatarID    *int
		wantName    string
		wantAvatar  int
	}{
		{"display_name only", name("Hero"), nil, "Hero", 0},
		{"avatar_id only", nil, avatarID(7), "initial", 7},
		{"both fields", name("Updated"), avatarID(3), "Updated", 3},
		{"neither field (no-op)", nil, nil, "initial", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStore()
			id := uuid.New()
			store.seedProfile(Profile{UserID: id, DisplayName: "initial", AvatarID: 0, Level: 1, EnergyUpdatedAt: time.Now()})
			svc := newTestService(store, time.Now)

			p, err := svc.UpdateProfile(context.Background(), id, tt.displayName, tt.avatarID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.DisplayName != tt.wantName {
				t.Errorf("DisplayName = %q, want %q", p.DisplayName, tt.wantName)
			}
			if p.AvatarID != tt.wantAvatar {
				t.Errorf("AvatarID = %d, want %d", p.AvatarID, tt.wantAvatar)
			}
		})
	}
}

func TestUpdateProfile_UnknownUser(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	svc := newTestService(store, time.Now)
	name := "ghost"
	_, err := svc.UpdateProfile(context.Background(), uuid.New(), &name, nil)
	if !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("err = %v, want ErrProfileNotFound", err)
	}
}

// ── SpendEnergy ───────────────────────────────────────────────────────────────

func TestSpendEnergy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		stored     int
		age        time.Duration
		spend      int
		wantEnergy int
		wantErr    error
	}{
		{"spend from full", EnergyMax, 0, 1, EnergyMax - 1, nil},
		{"spend with regen", 2, EnergyRegenInterval, 2, 1, nil},
		{"spend all", EnergyMax, 0, EnergyMax, 0, nil},
		{"insufficient", 0, 0, 1, 0, ErrInsufficientEnergy},
		{"regen not enough", 0, EnergyRegenInterval - time.Second, 1, 0, ErrInsufficientEnergy},
		{"zero amount rejected", EnergyMax, 0, 0, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			now := time.Now()
			store := newFakeStore()
			id := uuid.New()
			store.seedProfile(Profile{
				UserID:          id,
				Energy:          tt.stored,
				EnergyUpdatedAt: now.Add(-tt.age),
				Level:           1,
			})
			svc := newTestService(store, func() time.Time { return now })

			if tt.spend <= 0 {
				_, err := svc.SpendEnergy(context.Background(), id, tt.spend)
				if err == nil {
					t.Fatal("expected error for non-positive amount, got nil")
				}
				return
			}

			p, err := svc.SpendEnergy(context.Background(), id, tt.spend)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Energy != tt.wantEnergy {
				t.Errorf("Energy = %d, want %d", p.Energy, tt.wantEnergy)
			}
		})
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

// TestSpendGoldConcurrent verifies that concurrent SpendGold calls do not
// produce phantom gold: the total debited must not exceed the initial balance.
// The fake store's mutex serialises mutations, mirroring the FOR UPDATE lock
// that the real store would use.
func TestSpendGoldConcurrent(t *testing.T) {
	t.Parallel()

	const (
		startGold   = int64(1000)
		spendAmount = int64(10)
		goroutines  = 200
	)

	store := newFakeStore()
	id := uuid.New()
	store.seedProfile(Profile{UserID: id, Gold: startGold, Level: 1, EnergyUpdatedAt: time.Now()})
	svc := newTestService(store, time.Now)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for range goroutines {
		wg.Go(func() {
			_, err := svc.SpendGold(context.Background(), id, spendAmount)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	p, err := store.GetProfile(context.Background(), id)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	want := startGold - int64(successCount)*spendAmount
	if p.Gold != want {
		t.Errorf("Gold = %d after %d successes, want %d (started at %d)",
			p.Gold, successCount, want, startGold)
	}
	if p.Gold < 0 {
		t.Errorf("Gold went negative: %d", p.Gold)
	}
}

// TestSpendDiamondsConcurrent mirrors TestSpendGoldConcurrent for diamonds.
func TestSpendDiamondsConcurrent(t *testing.T) {
	t.Parallel()

	const (
		startDiamonds = int64(500)
		spendAmount   = int64(5)
		goroutines    = 200
	)

	store := newFakeStore()
	id := uuid.New()
	store.seedProfile(Profile{UserID: id, Diamonds: startDiamonds, Level: 1, EnergyUpdatedAt: time.Now()})
	svc := newTestService(store, time.Now)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for range goroutines {
		wg.Go(func() {
			_, err := svc.SpendDiamonds(context.Background(), id, spendAmount)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	p, err := store.GetProfile(context.Background(), id)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}

	want := startDiamonds - int64(successCount)*spendAmount
	if p.Diamonds != want {
		t.Errorf("Diamonds = %d after %d successes, want %d", p.Diamonds, successCount, want)
	}
	if p.Diamonds < 0 {
		t.Errorf("Diamonds went negative: %d", p.Diamonds)
	}
}

// ── Profile not found ─────────────────────────────────────────────────────────

func TestResourceService_UnknownUser(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	svc := newTestService(store, time.Now)
	id := uuid.New() // never seeded

	t.Run("GetProfile", func(t *testing.T) {
		t.Parallel()
		_, err := svc.GetProfile(context.Background(), id)
		if !errors.Is(err, ErrProfileNotFound) {
			t.Errorf("err = %v, want ErrProfileNotFound", err)
		}
	})

	t.Run("AddGold", func(t *testing.T) {
		t.Parallel()
		_, err := svc.AddGold(context.Background(), id, 10)
		if !errors.Is(err, ErrProfileNotFound) {
			t.Errorf("err = %v, want ErrProfileNotFound", err)
		}
	})

	t.Run("SpendGold", func(t *testing.T) {
		t.Parallel()
		_, err := svc.SpendGold(context.Background(), id, 10)
		if !errors.Is(err, ErrProfileNotFound) {
			t.Errorf("err = %v, want ErrProfileNotFound", err)
		}
	})

	t.Run("SpendEnergy", func(t *testing.T) {
		t.Parallel()
		_, err := svc.SpendEnergy(context.Background(), id, 1)
		if !errors.Is(err, ErrProfileNotFound) {
			t.Errorf("err = %v, want ErrProfileNotFound", err)
		}
	})
}

// ── GetProfile energy hydration ───────────────────────────────────────────────

func TestGetProfile_EnergyHydrated(t *testing.T) {
	t.Parallel()
	now := time.Now()
	store := newFakeStore()
	id := uuid.New()
	store.seedProfile(Profile{
		UserID:          id,
		Energy:          1,
		EnergyUpdatedAt: now.Add(-2 * EnergyRegenInterval),
		Level:           1,
	})
	svc := newTestService(store, func() time.Time { return now })

	p, err := svc.GetProfile(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 stored + 2 ticks = 3, within EnergyMax.
	if p.Energy != 3 {
		t.Errorf("Energy = %d, want 3", p.Energy)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
