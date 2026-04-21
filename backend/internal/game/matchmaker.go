package game

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
	"github.com/70H4NN3S/TowerDefense/internal/ws"
)

// BucketSize is the trophy range per matchmaking bucket.
// Players within the same bucket are eligible to be matched.
const BucketSize int64 = 100

// bucketIdx maps a trophy count to a bucket index.
func bucketIdx(trophies int64) int64 {
	if trophies < 0 {
		return 0
	}
	return trophies / BucketSize
}

// MatchFoundPayload is the payload of ws.TypeMatchFound messages pushed to
// both players when a multiplayer match is ready.
type MatchFoundPayload struct {
	MatchID    string `json:"match_id"`
	MapID      string `json:"map_id"`
	Seed       int64  `json:"seed"`
	OpponentID string `json:"opponent_id"`
	// Role is 1 for player_one (owns the left half of the map) and 2 for
	// player_two (owns the right half).
	Role int `json:"role"`
}

// WSHub is the subset of ws.Hub consumed by Matchmaker and SessionManager.
// Declared consumer-side so tests can supply a fake without a real Hub.
type WSHub interface {
	Send(userID uuid.UUID, data []byte)
}

// SessionStarter is satisfied by SessionManager. The interface exists so
// Matchmaker tests can supply a fake without a real session loop.
type SessionStarter interface {
	Start(ctx context.Context, match Match, p1Trophies, p2Trophies int64)
}

// queueEntry holds a waiting player's matchmaking state.
type queueEntry struct {
	userID   uuid.UUID
	trophies int64
	mapID    string
}

// mmOpKind identifies the operation type in the matchmaker ops channel.
type mmOpKind int

const (
	mmOpJoin  mmOpKind = iota
	mmOpLeave          // fire-and-forget; no result
	mmOpSync           // used by tests to flush the ops queue
)

type mmOp struct {
	kind     mmOpKind
	entry    queueEntry
	resultCh chan error    // non-nil for mmOpJoin
	syncDone chan struct{} // non-nil for mmOpSync
}

// Matchmaker manages trophy-bucket queues and initiates multiplayer sessions.
// All queue mutations happen inside the single Run goroutine (no lock needed).
type Matchmaker struct {
	ops        chan mmOp
	queues     map[int64][]queueEntry // bucket → FIFO list of waiting players
	userBucket map[uuid.UUID]int64    // userID → bucket (for O(1) Leave lookup)
	store      MatchStore
	sessions   SessionStarter
	hub        WSHub
	now        func() time.Time

	// wg tracks active initMatch goroutines so SyncAll can wait for them.
	wg sync.WaitGroup
}

// NewMatchmaker constructs a Matchmaker.
func NewMatchmaker(store MatchStore, sessions SessionStarter, hub WSHub, now func() time.Time) *Matchmaker {
	return &Matchmaker{
		ops:        make(chan mmOp, 64),
		queues:     make(map[int64][]queueEntry),
		userBucket: make(map[uuid.UUID]int64),
		store:      store,
		sessions:   sessions,
		hub:        hub,
		now:        now,
	}
}

// Run processes matchmaking operations until ctx is cancelled.
// Must be called in its own goroutine.
func (m *Matchmaker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case op := <-m.ops:
			switch op.kind {
			case mmOpJoin:
				m.doJoin(ctx, op)
			case mmOpLeave:
				m.doLeave(op.entry.userID)
			case mmOpSync:
				close(op.syncDone)
			}
		}
	}
}

// Join enqueues userID (with their current trophies and chosen mapID) into
// the appropriate bucket. It blocks until the ops loop acknowledges the join
// (so it can return ErrAlreadyQueued synchronously).
func (m *Matchmaker) Join(ctx context.Context, userID uuid.UUID, trophies int64, mapID string) error {
	resultCh := make(chan error, 1)
	op := mmOp{
		kind:     mmOpJoin,
		entry:    queueEntry{userID: userID, trophies: trophies, mapID: mapID},
		resultCh: resultCh,
	}
	select {
	case m.ops <- op:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Leave removes userID from the queue (if present). It is idempotent and
// fire-and-forget: the HTTP handler returns 204 immediately.
func (m *Matchmaker) Leave(ctx context.Context, userID uuid.UUID) {
	select {
	case m.ops <- mmOp{kind: mmOpLeave, entry: queueEntry{userID: userID}}:
	case <-ctx.Done():
	}
}

// Sync blocks until the ops loop has processed all operations enqueued before
// this call. Intended for tests only.
func (m *Matchmaker) Sync() {
	done := make(chan struct{})
	m.ops <- mmOp{kind: mmOpSync, syncDone: done}
	<-done
}

// SyncAll calls Sync and then waits for all in-flight initMatch goroutines to
// finish. Intended for tests only.
func (m *Matchmaker) SyncAll() {
	m.Sync()
	m.wg.Wait()
}

// ── internal ops ──────────────────────────────────────────────────────────────

func (m *Matchmaker) doJoin(ctx context.Context, op mmOp) {
	entry := op.entry

	// Reject if the player is already waiting.
	if _, exists := m.userBucket[entry.userID]; exists {
		op.resultCh <- ErrAlreadyQueued
		return
	}

	b := bucketIdx(entry.trophies)

	// Check for a waiting opponent in the same bucket.
	if peers := m.queues[b]; len(peers) > 0 {
		peer := peers[0]
		m.queues[b] = peers[1:]
		if len(m.queues[b]) == 0 {
			delete(m.queues, b)
		}
		delete(m.userBucket, peer.userID)
		op.resultCh <- nil // join succeeded; match is being initiated

		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.initMatch(ctx, peer, entry)
		}()
		return
	}

	// No peer yet: enqueue.
	m.queues[b] = append(m.queues[b], entry)
	m.userBucket[entry.userID] = b
	op.resultCh <- nil
}

func (m *Matchmaker) doLeave(userID uuid.UUID) {
	b, exists := m.userBucket[userID]
	if !exists {
		return
	}
	delete(m.userBucket, userID)
	queue := m.queues[b]
	for i, e := range queue {
		if e.userID == userID {
			m.queues[b] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	if len(m.queues[b]) == 0 {
		delete(m.queues, b)
	}
}

// initMatch creates the DB row, starts the session, and pushes match.found
// to both players. It runs in its own goroutine so the ops loop stays fast.
func (m *Matchmaker) initMatch(ctx context.Context, p1, p2 queueEntry) {
	seed, err := generateSeed()
	if err != nil {
		slog.Error("matchmaker: generate seed", "err", err)
		return
	}

	match := Match{
		ID:        uuid.New(),
		PlayerOne: p1.userID,
		PlayerTwo: &p2.userID,
		Mode:      "ranked",
		MapID:     p1.mapID,
		Seed:      seed,
		StartedAt: m.now(),
		CreatedAt: m.now(),
	}
	match, err = m.store.InsertMatch(ctx, match)
	if err != nil {
		slog.Error("matchmaker: insert match", "err", err)
		return
	}

	m.sessions.Start(ctx, match, p1.trophies, p2.trophies)

	// Push match.found to player 1.
	if msg, merr := ws.Marshal(ws.TypeMatchFound, MatchFoundPayload{
		MatchID:    match.ID.String(),
		MapID:      match.MapID,
		Seed:       match.Seed,
		OpponentID: p2.userID.String(),
		Role:       1,
	}); merr == nil {
		m.hub.Send(p1.userID, msg)
	}

	// Push match.found to player 2.
	if msg, merr := ws.Marshal(ws.TypeMatchFound, MatchFoundPayload{
		MatchID:    match.ID.String(),
		MapID:      match.MapID,
		Seed:       match.Seed,
		OpponentID: p1.userID.String(),
		Role:       2,
	}); merr == nil {
		m.hub.Send(p2.userID, msg)
	}

	slog.Info("matchmaker: match initiated",
		"match_id", match.ID,
		"player_one", p1.userID,
		"player_two", p2.userID,
	)
}
