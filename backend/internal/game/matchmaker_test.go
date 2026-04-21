package game

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeSessions struct {
	mu      sync.Mutex
	started []Match
}

func (f *fakeSessions) Start(_ context.Context, m Match, _, _ int64) {
	f.mu.Lock()
	f.started = append(f.started, m)
	f.mu.Unlock()
}

func (f *fakeSessions) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.started)
}

func (f *fakeSessions) last() (Match, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.started) == 0 {
		return Match{}, false
	}
	return f.started[len(f.started)-1], true
}

type fakeWSHub struct {
	mu   sync.Mutex
	msgs map[uuid.UUID][][]byte
}

func newFakeWSHub() *fakeWSHub {
	return &fakeWSHub{msgs: make(map[uuid.UUID][][]byte)}
}

func (f *fakeWSHub) Send(userID uuid.UUID, data []byte) {
	f.mu.Lock()
	f.msgs[userID] = append(f.msgs[userID], data)
	f.mu.Unlock()
}

func (f *fakeWSHub) msgsFor(userID uuid.UUID) [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.msgs[userID]
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestMatchmaker(t *testing.T) (*Matchmaker, *fakeSessions, *fakeWSHub) {
	t.Helper()
	store := newFakeMatchStore()
	sessions := &fakeSessions{}
	hub := newFakeWSHub()
	mm := NewMatchmaker(store, sessions, hub, time.Now)
	return mm, sessions, hub
}

// ── unit tests ────────────────────────────────────────────────────────────────

func TestBucketIdx(t *testing.T) {
	t.Parallel()
	cases := []struct {
		trophies int64
		want     int64
	}{
		{0, 0},
		{99, 0},
		{100, 1},
		{199, 1},
		{300, 3},
		{-5, 0}, // clamped
	}
	for _, tt := range cases {
		got := bucketIdx(tt.trophies)
		if got != tt.want {
			t.Errorf("bucketIdx(%d) = %d, want %d", tt.trophies, got, tt.want)
		}
	}
}

// ── integration tests (require Run goroutine) ─────────────────────────────────

func TestMatchmaker_Join_IsQueued(t *testing.T) {
	t.Parallel()
	mm, _, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	uid := uuid.New()
	if err := mm.Join(t.Context(), uid, 50, "alpha"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	mm.Sync()
	if _, ok := mm.userBucket[uid]; !ok {
		t.Error("player not in userBucket after Join")
	}
}

func TestMatchmaker_Join_AlreadyQueued(t *testing.T) {
	t.Parallel()
	mm, _, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	uid := uuid.New()
	if err := mm.Join(t.Context(), uid, 50, "alpha"); err != nil {
		t.Fatalf("first Join: %v", err)
	}
	err := mm.Join(t.Context(), uid, 50, "alpha")
	if !errors.Is(err, ErrAlreadyQueued) {
		t.Errorf("second Join err = %v, want ErrAlreadyQueued", err)
	}
}

func TestMatchmaker_Leave_RemovesPlayer(t *testing.T) {
	t.Parallel()
	mm, _, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	uid := uuid.New()
	if err := mm.Join(t.Context(), uid, 50, "alpha"); err != nil {
		t.Fatalf("Join: %v", err)
	}
	mm.Leave(t.Context(), uid)
	mm.Sync()

	if _, ok := mm.userBucket[uid]; ok {
		t.Error("player still in userBucket after Leave")
	}
}

func TestMatchmaker_Leave_NotQueued_IsNoop(t *testing.T) {
	t.Parallel()
	mm, _, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	// Leaving when not queued must not panic or error.
	mm.Leave(t.Context(), uuid.New())
	mm.Sync()
}

func TestMatchmaker_TwoPlayersMatch(t *testing.T) {
	t.Parallel()
	mm, sessions, hub := newTestMatchmaker(t)
	go mm.Run(t.Context())

	p1, p2 := uuid.New(), uuid.New()
	if err := mm.Join(t.Context(), p1, 50, "alpha"); err != nil {
		t.Fatalf("p1 Join: %v", err)
	}
	if err := mm.Join(t.Context(), p2, 75, "alpha"); err != nil {
		t.Fatalf("p2 Join: %v", err)
	}
	mm.SyncAll()

	// One session should have been started.
	if sessions.count() != 1 {
		t.Fatalf("sessions started = %d, want 1", sessions.count())
	}

	// Both players should have received match.found.
	for _, uid := range []uuid.UUID{p1, p2} {
		msgs := hub.msgsFor(uid)
		if len(msgs) == 0 {
			t.Errorf("no match.found for %s", uid)
			continue
		}
		var env struct {
			Type string          `json:"type"`
			V    int             `json:"v"`
			P    json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(msgs[0], &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if env.Type != "match.found" {
			t.Errorf("type = %q, want %q", env.Type, "match.found")
		}
	}

	// Neither player should remain in the queue.
	if _, ok := mm.userBucket[p1]; ok {
		t.Error("p1 still queued")
	}
	if _, ok := mm.userBucket[p2]; ok {
		t.Error("p2 still queued")
	}
}

func TestMatchmaker_DifferentBuckets_NoMatch(t *testing.T) {
	t.Parallel()
	mm, sessions, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	// p1: bucket 0 (0–99), p2: bucket 5 (500–599)
	p1, p2 := uuid.New(), uuid.New()
	if err := mm.Join(t.Context(), p1, 50, "alpha"); err != nil {
		t.Fatalf("p1 Join: %v", err)
	}
	if err := mm.Join(t.Context(), p2, 550, "alpha"); err != nil {
		t.Fatalf("p2 Join: %v", err)
	}
	mm.SyncAll()

	if sessions.count() != 0 {
		t.Errorf("sessions started = %d, want 0 (different buckets)", sessions.count())
	}
	// Both should remain queued.
	if _, ok := mm.userBucket[p1]; !ok {
		t.Error("p1 should still be queued")
	}
	if _, ok := mm.userBucket[p2]; !ok {
		t.Error("p2 should still be queued")
	}
}

func TestMatchmaker_BucketFairness_ArrivalOrder(t *testing.T) {
	t.Parallel()
	mm, sessions, _ := newTestMatchmaker(t)
	go mm.Run(t.Context())

	// p1 joins and waits in the queue.
	p1 := uuid.New()
	if err := mm.Join(t.Context(), p1, 50, "alpha"); err != nil {
		t.Fatalf("p1 Join: %v", err)
	}
	mm.Sync() // guarantee p1 is in the queue before p2 arrives

	// p2 joins: triggers a match with p1 (the first waiter).
	p2 := uuid.New()
	if err := mm.Join(t.Context(), p2, 75, "alpha"); err != nil {
		t.Fatalf("p2 Join: %v", err)
	}
	mm.SyncAll()

	if sessions.count() != 1 {
		t.Fatalf("sessions = %d, want 1", sessions.count())
	}

	m, _ := sessions.last()
	// The waiting player (p1) becomes player_one; the trigger (p2) is player_two.
	if m.PlayerOne != p1 {
		t.Errorf("player_one = %s, want %s (arrival order)", m.PlayerOne, p1)
	}
	if m.PlayerTwo == nil || *m.PlayerTwo != p2 {
		t.Errorf("player_two = %v, want %s", m.PlayerTwo, p2)
	}
}

func TestMatchmaker_MatchedPayload_Roles(t *testing.T) {
	t.Parallel()
	mm, _, hub := newTestMatchmaker(t)
	go mm.Run(t.Context())

	p1, p2 := uuid.New(), uuid.New()
	_ = mm.Join(t.Context(), p1, 50, "alpha")
	_ = mm.Join(t.Context(), p2, 50, "alpha")
	mm.SyncAll()

	decode := func(uid uuid.UUID) MatchFoundPayload {
		t.Helper()
		msgs := hub.msgsFor(uid)
		if len(msgs) == 0 {
			t.Fatalf("no messages for %s", uid)
		}
		var env struct {
			P MatchFoundPayload `json:"payload"`
		}
		if err := json.Unmarshal(msgs[0], &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return env.P
	}

	pp1 := decode(p1)
	pp2 := decode(p2)

	if pp1.Role != 1 {
		t.Errorf("p1 role = %d, want 1", pp1.Role)
	}
	if pp2.Role != 2 {
		t.Errorf("p2 role = %d, want 2", pp2.Role)
	}
	if pp1.OpponentID != p2.String() {
		t.Errorf("p1 opponent = %s, want %s", pp1.OpponentID, p2)
	}
	if pp2.OpponentID != p1.String() {
		t.Errorf("p2 opponent = %s, want %s", pp2.OpponentID, p1)
	}
}
