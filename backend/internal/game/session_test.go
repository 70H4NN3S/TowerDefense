package game

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/game/sim"
	"github.com/johannesniedens/towerdefense/internal/uuid"
	"github.com/johannesniedens/towerdefense/internal/ws"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestSessionManager returns a SessionManager wired with fakes and a fast
// tick rate suitable for unit tests.
func newTestSessionManager(t *testing.T, tickDt time.Duration) (*SessionManager, *fakeMatchStore, *fakeMatchResourcer, *fakeWSHub) {
	t.Helper()
	store := newFakeMatchStore()
	hub := newFakeWSHub()
	res := newFakeMatchResourcer()
	mgr := NewSessionManager(store, res, hub, time.Now)
	mgr.tickDt = tickDt
	return mgr, store, res, hub
}

// newTestMatch returns a Match pre-inserted into store with both players.
func newTestMatch(t *testing.T, store *fakeMatchStore, p1, p2 uuid.UUID) Match {
	t.Helper()
	p2copy := p2
	m := Match{
		ID:        uuid.New(),
		PlayerOne: p1,
		PlayerTwo: &p2copy,
		Mode:      "ranked",
		MapID:     "alpha",
		Seed:      12345,
		StartedAt: time.Now(),
		CreatedAt: time.Now(),
	}
	inserted, err := store.InsertMatch(context.Background(), m)
	if err != nil {
		t.Fatalf("newTestMatch: insert: %v", err)
	}
	return inserted
}

// buildSession constructs a Session in-process (no goroutine) with the given
// initial state. Useful for testing individual methods.
func buildSession(mgr *SessionManager, match Match, p1, p2 uuid.UUID, state sim.State, halfCols int) *Session {
	return &Session{
		match: match,
		p1: sessionPlayer{
			userID: p1,
			minCol: 0,
			maxCol: halfCols,
		},
		p2: sessionPlayer{
			userID: p2,
			minCol: halfCols,
			maxCol: halfCols * 2, // symmetric map
		},
		state:   state,
		inputCh: make(chan sessionInput, 64),
		mgr:     mgr,
		tickDt:  mgr.tickDt,
	}
}

// waitCondition polls cond every 5 ms for up to timeout.
func waitCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// ── mergeInputs ───────────────────────────────────────────────────────────────

func TestSession_MergeInputs_SideValidation(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	// p1 places a tower on col=0 (their side) and col=halfCols (p2's side).
	inp := sim.Input{
		PlaceTowers: []sim.PlaceTower{
			{TemplateID: "archer", Tile: sim.Tile{Col: 0, Row: 0}, GoldCost: 10, Damage: 10, Range: 3, Rate: 1},
			{TemplateID: "archer", Tile: sim.Tile{Col: halfCols, Row: 0}, GoldCost: 10, Damage: 10, Range: 3, Rate: 1},
		},
	}

	combined := sess.mergeInputs([]sessionInput{{userID: p1, seq: 1, input: inp}})

	if len(combined.PlaceTowers) != 1 {
		t.Fatalf("place_towers = %d, want 1 (cross-side tower filtered)", len(combined.PlaceTowers))
	}
	if combined.PlaceTowers[0].Tile.Col != 0 {
		t.Errorf("surviving tower col = %d, want 0", combined.PlaceTowers[0].Tile.Col)
	}
}

func TestSession_MergeInputs_SequenceFilter(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	tower := sim.PlaceTower{
		TemplateID: "archer", Tile: sim.Tile{Col: 0, Row: 0},
		GoldCost: 10, Damage: 10, Range: 3, Rate: 1,
	}
	inputs := []sessionInput{
		{userID: p1, seq: 2, input: sim.Input{PlaceTowers: []sim.PlaceTower{tower}}},
		{userID: p1, seq: 1, input: sim.Input{PlaceTowers: []sim.PlaceTower{tower}}}, // stale
		{userID: p1, seq: 2, input: sim.Input{PlaceTowers: []sim.PlaceTower{tower}}}, // duplicate
		{userID: p1, seq: 3, input: sim.Input{PlaceTowers: []sim.PlaceTower{tower}}}, // new
	}

	combined := sess.mergeInputs(inputs)

	// Only seqs 2 and 3 should pass (first occurrence of 2, then 3).
	if len(combined.PlaceTowers) != 2 {
		t.Fatalf("place_towers = %d, want 2 (stale and duplicate filtered)", len(combined.PlaceTowers))
	}
}

func TestSession_MergeInputs_UnknownPlayerDropped(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	stranger := uuid.New()
	inp := sim.Input{
		PlaceTowers: []sim.PlaceTower{
			{TemplateID: "archer", Tile: sim.Tile{Col: 0, Row: 0}, GoldCost: 10, Damage: 10, Range: 3, Rate: 1},
		},
	}
	combined := sess.mergeInputs([]sessionInput{{userID: stranger, seq: 1, input: inp}})

	if len(combined.PlaceTowers) != 0 {
		t.Errorf("unexpected towers from unknown player: %d", len(combined.PlaceTowers))
	}
}

// ── endMatch ─────────────────────────────────────────────────────────────────

func TestSession_EndMatch_Victory(t *testing.T) {
	t.Parallel()

	mgr, store, res, hub := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	// Pre-seed profiles so resource calls succeed.
	res.profiles[p1] = Profile{UserID: p1, Gold: 0, Trophies: 100}
	res.profiles[p2] = Profile{UserID: p2, Gold: 0, Trophies: 100}

	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	state.Gold = 200 // some gold from kills
	state.GateHP = 10
	state.WaveIdx = len(waves) // victory condition already met
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	sess.endMatch(context.Background())

	// Match should be ended with PlayerOne as winner.
	ended, err := store.GetMatch(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("get match: %v", err)
	}
	if ended.EndedAt == nil {
		t.Error("EndedAt is nil after endMatch")
	}
	if ended.Winner == nil || *ended.Winner != p1 {
		t.Errorf("winner = %v, want %s", ended.Winner, p1)
	}

	// Both players should have received gold and trophies.
	for _, uid := range []uuid.UUID{p1, p2} {
		p := res.profiles[uid]
		if p.Gold != 200 {
			t.Errorf("%s gold = %d, want 200", uid, p.Gold)
		}
		if p.Trophies != 100+trophyRewardMultiplayer {
			t.Errorf("%s trophies = %d, want %d", uid, p.Trophies, 100+trophyRewardMultiplayer)
		}
	}

	// Both players should have received match.ended.
	for _, uid := range []uuid.UUID{p1, p2} {
		msgs := hub.msgsFor(uid)
		if len(msgs) == 0 {
			t.Errorf("no messages for %s", uid)
			continue
		}
		var env struct {
			Type string            `json:"type"`
			P    MatchEndedPayload `json:"payload"`
		}
		if err := json.Unmarshal(msgs[0], &env); err != nil {
			t.Fatalf("unmarshal match.ended: %v", err)
		}
		if env.Type != ws.TypeMatchEnded {
			t.Errorf("type = %q, want %q", env.Type, ws.TypeMatchEnded)
		}
		if env.P.GoldAwarded != 200 {
			t.Errorf("gold_awarded = %d, want 200", env.P.GoldAwarded)
		}
		if env.P.TrophyDelta != trophyRewardMultiplayer {
			t.Errorf("trophy_delta = %d, want %d", env.P.TrophyDelta, trophyRewardMultiplayer)
		}
		if env.P.WinnerID != p1.String() {
			t.Errorf("winner_id = %q, want %q", env.P.WinnerID, p1.String())
		}
	}

	// Players should be removed from the manager.
	mgr.mu.RLock()
	_, stillInP1 := mgr.byUser[p1]
	_, stillInP2 := mgr.byUser[p2]
	mgr.mu.RUnlock()
	if stillInP1 || stillInP2 {
		t.Error("players still registered after match end")
	}
}

func TestSession_EndMatch_Defeat(t *testing.T) {
	t.Parallel()

	mgr, store, res, hub := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	res.profiles[p1] = Profile{UserID: p1, Gold: 0, Trophies: 100}
	res.profiles[p2] = Profile{UserID: p2, Gold: 0, Trophies: 100}

	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	state.Gold = 50
	state.GateHP = 0 // defeat condition
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	sess.endMatch(context.Background())

	// No trophies on defeat.
	for _, uid := range []uuid.UUID{p1, p2} {
		if res.profiles[uid].Trophies != 100 {
			t.Errorf("%s trophies changed on defeat: got %d, want 100", uid, res.profiles[uid].Trophies)
		}
	}

	// Gold is still awarded.
	for _, uid := range []uuid.UUID{p1, p2} {
		if res.profiles[uid].Gold != 50 {
			t.Errorf("%s gold = %d, want 50", uid, res.profiles[uid].Gold)
		}
	}

	// match.ended should have empty winner_id and zero trophy_delta.
	msgs := hub.msgsFor(p1)
	if len(msgs) == 0 {
		t.Fatal("no match.ended for p1")
	}
	var env struct {
		P MatchEndedPayload `json:"payload"`
	}
	if err := json.Unmarshal(msgs[0], &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.P.WinnerID != "" {
		t.Errorf("winner_id = %q, want empty on defeat", env.P.WinnerID)
	}
	if env.P.TrophyDelta != 0 {
		t.Errorf("trophy_delta = %d, want 0 on defeat", env.P.TrophyDelta)
	}
}

// ── SessionManager ────────────────────────────────────────────────────────────

func TestSessionManager_Start_UnknownMap(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	p2copy := p2
	m := Match{
		ID:        uuid.New(),
		PlayerOne: p1,
		PlayerTwo: &p2copy,
		MapID:     "nonexistent-map",
	}
	_, _ = store.InsertMatch(context.Background(), m)

	// Must not panic; players must NOT be registered.
	mgr.Start(context.Background(), m, 100, 100)

	mgr.mu.RLock()
	_, ok := mgr.byUser[p1]
	mgr.mu.RUnlock()
	if ok {
		t.Error("player registered despite unknown map")
	}
}

func TestSessionManager_Start_RegistersBothPlayers(t *testing.T) {
	t.Parallel()

	mgr, store, res, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	res.profiles[p1] = Profile{UserID: p1}
	res.profiles[p2] = Profile{UserID: p2}
	m := newTestMatch(t, store, p1, p2)

	mgr.Start(t.Context(), m, 100, 150)

	mgr.mu.RLock()
	s1, ok1 := mgr.byUser[p1]
	s2, ok2 := mgr.byUser[p2]
	mgr.mu.RUnlock()

	if !ok1 || !ok2 {
		t.Fatal("players not registered after Start")
	}
	if s1 != s2 {
		t.Error("p1 and p2 registered to different sessions")
	}
}

func TestSessionManager_Dispatch_RoutesInput(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	m := newTestMatch(t, store, p1, p2)

	// Build session without starting the run goroutine so nobody races for inputCh.
	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	mgr.mu.Lock()
	mgr.byUser[p1] = sess
	mgr.byUser[p2] = sess
	mgr.mu.Unlock()

	payload, _ := json.Marshal(MatchInputPayload{Seq: 1, Input: sim.Input{}})
	mgr.Dispatch(p1, ws.TypeMatchInput, payload)

	select {
	case inp := <-sess.inputCh:
		if inp.userID != p1 {
			t.Errorf("input userID = %s, want %s", inp.userID, p1)
		}
		if inp.seq != 1 {
			t.Errorf("seq = %d, want 1", inp.seq)
		}
	default:
		t.Error("no input delivered to session after Dispatch")
	}
}

func TestSessionManager_Dispatch_UnknownPlayerDropped(t *testing.T) {
	t.Parallel()

	mgr, _, _, _ := newTestSessionManager(t, defaultTickDt)
	payload, _ := json.Marshal(MatchInputPayload{Seq: 1, Input: sim.Input{}})

	// Must not panic for a player not in any session.
	mgr.Dispatch(uuid.New(), ws.TypeMatchInput, payload)
}

func TestSessionManager_Dispatch_WrongTypeIgnored(t *testing.T) {
	t.Parallel()

	mgr, store, _, _ := newTestSessionManager(t, defaultTickDt)
	p1, p2 := uuid.New(), uuid.New()
	m := newTestMatch(t, store, p1, p2)

	// Register session without starting run goroutine.
	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	halfCols := simMap.Cols / 2
	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	mgr.mu.Lock()
	mgr.byUser[p1] = sess
	mgr.mu.Unlock()

	mgr.Dispatch(p1, ws.TypeMatchSnapshot, json.RawMessage(`{}`))

	select {
	case <-sess.inputCh:
		t.Error("input delivered for non-match.input message type")
	default:
	}
}

// ── lifecycle integration ─────────────────────────────────────────────────────

// TestSession_Run_VictoryTerminates verifies the session's run loop exits and
// cleans up when a victory condition is detected on the first tick.
func TestSession_Run_VictoryTerminates(t *testing.T) {
	t.Parallel()

	mgr, store, res, hub := newTestSessionManager(t, 5*time.Millisecond)
	p1, p2 := uuid.New(), uuid.New()
	res.profiles[p1] = Profile{UserID: p1, Gold: 0, Trophies: 0}
	res.profiles[p2] = Profile{UserID: p2, Gold: 0, Trophies: 0}
	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	state := sim.InitialState(simMap, waves)
	state.Gold = 100
	state.GateHP = 5
	state.WaveIdx = len(waves) // already victorious
	halfCols := simMap.Cols / 2

	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	// Register players so endMatch can remove them.
	mgr.mu.Lock()
	mgr.byUser[p1] = sess
	mgr.byUser[p2] = sess
	mgr.mu.Unlock()

	go sess.run(t.Context())

	// Wait for players to be deregistered (signals endMatch ran).
	waitCondition(t, time.Second, func() bool {
		mgr.mu.RLock()
		defer mgr.mu.RUnlock()
		_, ok1 := mgr.byUser[p1]
		_, ok2 := mgr.byUser[p2]
		return !ok1 && !ok2
	})

	// match.ended must have been sent to both players.
	for _, uid := range []uuid.UUID{p1, p2} {
		if len(hub.msgsFor(uid)) == 0 {
			t.Errorf("no messages for %s after victory", uid)
		}
	}
}

// TestSession_Run_DefeatTerminates verifies defeat (GateHP=0 after a step).
// We inject a state with GateHP=1 and one monster that will reach the gate
// in a single tick at a very high speed.
func TestSession_Run_DefeatTerminates(t *testing.T) {
	t.Parallel()

	mgr, store, res, hub := newTestSessionManager(t, 5*time.Millisecond)
	p1, p2 := uuid.New(), uuid.New()
	res.profiles[p1] = Profile{UserID: p1}
	res.profiles[p2] = Profile{UserID: p2}
	m := newTestMatch(t, store, p1, p2)

	simMap, waves, _ := sim.LookupMap("alpha")
	// Path length for alpha is ~22 units. A speed of 10000 world-units/sec
	// over a 5ms tick (0.005 sim-seconds) = 50 world-units: enough to cross
	// the entire map in one tick.
	state := sim.InitialState(simMap, waves)
	state.GateHP = 1
	state.Gold = 0
	// Inject a fast monster directly so it reaches the gate in one step.
	state.Monsters = []sim.Monster{
		{ID: 1, HP: 100, MaxHP: 100, Speed: 10000, Reward: 0, Progress: 0, Alive: true},
	}
	halfCols := simMap.Cols / 2

	sess := buildSession(mgr, m, p1, p2, state, halfCols)

	mgr.mu.Lock()
	mgr.byUser[p1] = sess
	mgr.byUser[p2] = sess
	mgr.mu.Unlock()

	go sess.run(t.Context())

	waitCondition(t, time.Second, func() bool {
		mgr.mu.RLock()
		defer mgr.mu.RUnlock()
		_, ok1 := mgr.byUser[p1]
		_, ok2 := mgr.byUser[p2]
		return !ok1 && !ok2
	})

	for _, uid := range []uuid.UUID{p1, p2} {
		msgs := hub.msgsFor(uid)
		found := false
		for _, msg := range msgs {
			var env struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(msg, &env); err == nil && env.Type == ws.TypeMatchEnded {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no match.ended for %s after defeat", uid)
		}
	}
}
