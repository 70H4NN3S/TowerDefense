package game

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/game/sim"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
	"github.com/70H4NN3S/TowerDefense/internal/ws"
)

const (
	// defaultTickDt is the real-time interval between simulation ticks.
	defaultTickDt = 200 * time.Millisecond

	// trophyRewardMultiplayer is the trophy gain awarded to both players on
	// a co-op victory. Trophy loss on defeat is deferred (ELO-lite v2).
	trophyRewardMultiplayer int64 = 25

	// startingGold is the gold balance given to both players at match start.
	startingGold int64 = 150

	// startingGateHP is the gate hit-points the players must defend.
	startingGateHP int64 = 20
)

// ── wire types ────────────────────────────────────────────────────────────────

// MatchInputPayload is the client-side payload for ws.TypeMatchInput messages.
type MatchInputPayload struct {
	Seq   int64     `json:"seq"`
	Input sim.Input `json:"input"`
}

// MatchSnapshotPayload is the server-side payload for ws.TypeMatchSnapshot messages.
type MatchSnapshotPayload struct {
	Tick  int64     `json:"tick"`
	State sim.State `json:"state"`
}

// MatchEndedPayload is the server-side payload for ws.TypeMatchEnded messages.
type MatchEndedPayload struct {
	WinnerID    string `json:"winner_id,omitempty"`
	GoldAwarded int64  `json:"gold_awarded"`
	TrophyDelta int64  `json:"trophy_delta"`
}

// ── session internals ─────────────────────────────────────────────────────────

// sessionPlayer holds per-player state within a session.
type sessionPlayer struct {
	userID   uuid.UUID
	trophies int64
	minCol   int   // inclusive lower bound of the player's tile columns
	maxCol   int   // exclusive upper bound of the player's tile columns
	nextSeq  int64 // highest seq seen; inputs with seq <= nextSeq are dropped
}

// sessionInput is a player action message delivered from the WS hub.
type sessionInput struct {
	userID uuid.UUID
	seq    int64
	input  sim.Input
}

// Session is one in-flight co-op multiplayer session. The run goroutine owns
// all mutable state; the only concurrency-safe entry point is SendInput.
type Session struct {
	match   Match
	p1, p2  sessionPlayer
	state   sim.State
	inputCh chan sessionInput
	mgr     *SessionManager
	tickDt  time.Duration
}

// SendInput delivers a player input to the session's run loop from any goroutine.
// If the channel is full (client flooding) the message is silently dropped; the
// client will reconcile against the next snapshot.
func (s *Session) SendInput(userID uuid.UUID, seq int64, input sim.Input) {
	select {
	case s.inputCh <- sessionInput{userID: userID, seq: seq, input: input}:
	default:
	}
}

// run is the session's main loop. It must be called in its own goroutine.
func (s *Session) run(ctx context.Context) {
	ticker := time.NewTicker(s.tickDt)
	defer ticker.Stop()

	var pending []sessionInput

	for {
		select {
		case <-ctx.Done():
			return
		case inp := <-s.inputCh:
			pending = append(pending, inp)
		case <-ticker.C:
			// Drain any inputs that arrived between ticks.
		drain:
			for {
				select {
				case inp := <-s.inputCh:
					pending = append(pending, inp)
				default:
					break drain
				}
			}

			combined := s.mergeInputs(pending)
			pending = pending[:0]

			s.state = sim.Step(s.state, combined, s.tickDt.Seconds())
			s.sendSnapshot()

			if s.state.GateHP == 0 || s.isVictory() {
				s.endMatch(ctx)
				return
			}
		}
	}
}

// mergeInputs combines inputs from both players, filtering each player's
// PlaceTower commands to their own half of the map and dropping stale seqs.
func (s *Session) mergeInputs(pending []sessionInput) sim.Input {
	var combined sim.Input
	for _, inp := range pending {
		var player *sessionPlayer
		switch inp.userID {
		case s.p1.userID:
			player = &s.p1
		case s.p2.userID:
			player = &s.p2
		default:
			continue
		}
		if inp.seq <= player.nextSeq {
			continue // stale or duplicate
		}
		player.nextSeq = inp.seq

		for _, pt := range inp.input.PlaceTowers {
			if pt.Tile.Col >= player.minCol && pt.Tile.Col < player.maxCol {
				combined.PlaceTowers = append(combined.PlaceTowers, pt)
			}
		}
	}
	return combined
}

func (s *Session) isVictory() bool {
	return s.state.WaveIdx >= len(s.state.Waves)
}

func (s *Session) sendSnapshot() {
	msg, err := ws.Marshal(ws.TypeMatchSnapshot, MatchSnapshotPayload{
		Tick:  s.state.Tick,
		State: s.state,
	})
	if err != nil {
		slog.Error("session: marshal snapshot", "err", err, "match_id", s.match.ID)
		return
	}
	s.mgr.hub.Send(s.p1.userID, msg)
	s.mgr.hub.Send(s.p2.userID, msg)
}

// endMatch finalises the match: persists the result, awards resources to both
// players, pushes match.ended, and removes the session from the manager.
func (s *Session) endMatch(ctx context.Context) {
	victory := s.isVictory()

	var winnerID *uuid.UUID
	var trophyDelta int64

	if victory {
		trophyDelta = trophyRewardMultiplayer
		winnerID = &s.match.PlayerOne // both co-op; record p1 as canonical winner
	}

	// Gold awarded = whatever the shared sim gold balance is at game end.
	goldAwarded := s.state.Gold

	endedAt := s.mgr.now()
	match, err := s.mgr.store.EndMatch(ctx, s.match.ID, winnerID, endedAt)
	if err != nil {
		slog.Error("session: end match", "err", err, "match_id", s.match.ID)
	} else {
		s.match = match
	}

	for _, p := range []sessionPlayer{s.p1, s.p2} {
		if goldAwarded > 0 {
			if _, err := s.mgr.resources.AddGold(ctx, p.userID, goldAwarded); err != nil {
				slog.Error("session: award gold", "err", err, "user_id", p.userID)
			}
		}
		if trophyDelta > 0 {
			if _, err := s.mgr.resources.AddTrophies(ctx, p.userID, trophyDelta); err != nil {
				slog.Error("session: award trophies", "err", err, "user_id", p.userID)
			}
		}
	}

	winnerStr := ""
	if winnerID != nil {
		winnerStr = winnerID.String()
	}
	msg, err := ws.Marshal(ws.TypeMatchEnded, MatchEndedPayload{
		WinnerID:    winnerStr,
		GoldAwarded: goldAwarded,
		TrophyDelta: trophyDelta,
	})
	if err != nil {
		slog.Error("session: marshal match.ended", "err", err, "match_id", s.match.ID)
	} else {
		s.mgr.hub.Send(s.p1.userID, msg)
		s.mgr.hub.Send(s.p2.userID, msg)
	}

	slog.Info("session: match ended",
		"match_id", s.match.ID,
		"victory", victory,
		"gold_awarded", goldAwarded,
		"trophy_delta", trophyDelta,
	)

	s.mgr.remove(s.p1.userID, s.p2.userID)
}

// ── SessionManager ────────────────────────────────────────────────────────────

// SessionManager creates and tracks all in-flight multiplayer sessions.
// It implements SessionStarter so Matchmaker can start sessions without
// knowing their internals.
type SessionManager struct {
	mu        sync.RWMutex
	byUser    map[uuid.UUID]*Session
	hub       WSHub
	store     MatchStore
	resources MatchResourcer
	tickDt    time.Duration
	now       func() time.Time
}

// NewSessionManager constructs a SessionManager.
func NewSessionManager(store MatchStore, resources MatchResourcer, hub WSHub, now func() time.Time) *SessionManager {
	return &SessionManager{
		byUser:    make(map[uuid.UUID]*Session),
		hub:       hub,
		store:     store,
		resources: resources,
		tickDt:    defaultTickDt,
		now:       now,
	}
}

// Start implements SessionStarter. It initialises the authoritative sim state,
// registers both players, and launches the session's run goroutine.
func (m *SessionManager) Start(ctx context.Context, match Match, p1Trophies, p2Trophies int64) {
	simMap, waves, ok := sim.LookupMap(match.MapID)
	if !ok {
		slog.Error("session manager: unknown map", "map_id", match.MapID, "match_id", match.ID)
		return
	}

	halfCols := simMap.Cols / 2
	state := sim.InitialState(simMap, waves)
	state.Gold = startingGold
	state.GateHP = startingGateHP

	p2ID := uuid.UUID("")
	if match.PlayerTwo != nil {
		p2ID = *match.PlayerTwo
	}

	sess := &Session{
		match: match,
		p1: sessionPlayer{
			userID:   match.PlayerOne,
			trophies: p1Trophies,
			minCol:   0,
			maxCol:   halfCols,
		},
		p2: sessionPlayer{
			userID:   p2ID,
			trophies: p2Trophies,
			minCol:   halfCols,
			maxCol:   simMap.Cols,
		},
		state:   state,
		inputCh: make(chan sessionInput, 64),
		mgr:     m,
		tickDt:  m.tickDt,
	}

	m.mu.Lock()
	m.byUser[match.PlayerOne] = sess
	if p2ID != "" {
		m.byUser[p2ID] = sess
	}
	m.mu.Unlock()

	go sess.run(ctx)
}

// Dispatch routes ws.TypeMatchInput messages from the WS hub to the correct
// session. It is intended to be wired as (or called from) the hub's DispatchFunc.
func (m *SessionManager) Dispatch(userID uuid.UUID, msgType string, payload json.RawMessage) {
	if msgType != ws.TypeMatchInput {
		return
	}

	m.mu.RLock()
	sess, ok := m.byUser[userID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	var p MatchInputPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		slog.Warn("session: bad match.input payload", "err", err, "user_id", userID)
		return
	}

	sess.SendInput(userID, p.Seq, p.Input)
}

// remove deregisters the given players from the active-session index.
func (m *SessionManager) remove(userIDs ...uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range userIDs {
		delete(m.byUser, id)
	}
}
