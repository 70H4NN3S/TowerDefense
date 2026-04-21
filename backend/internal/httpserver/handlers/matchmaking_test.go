package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeMatchQueue ────────────────────────────────────────────────────────────

type fakeMatchQueue struct {
	joinErr error
	joined  []uuid.UUID
	left    []uuid.UUID
}

func (f *fakeMatchQueue) Join(_ context.Context, userID uuid.UUID, _ int64, _ string) error {
	if f.joinErr != nil {
		return f.joinErr
	}
	f.joined = append(f.joined, userID)
	return nil
}

func (f *fakeMatchQueue) Leave(_ context.Context, userID uuid.UUID) {
	f.left = append(f.left, userID)
}

// ── fakeProfileReader ─────────────────────────────────────────────────────────

type fakeProfileReader struct {
	profiles map[uuid.UUID]game.Profile
	readErr  error
}

func newFakeProfileReader(users ...uuid.UUID) *fakeProfileReader {
	r := &fakeProfileReader{profiles: make(map[uuid.UUID]game.Profile)}
	for _, u := range users {
		r.profiles[u] = game.Profile{UserID: u, Trophies: 100}
	}
	return r
}

func (f *fakeProfileReader) GetProfile(_ context.Context, userID uuid.UUID) (game.Profile, error) {
	if f.readErr != nil {
		return game.Profile{}, f.readErr
	}
	p, ok := f.profiles[userID]
	if !ok {
		return game.Profile{}, game.ErrProfileNotFound
	}
	return p, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newMatchmakingMux(queue *fakeMatchQueue, profile *fakeProfileReader) *http.ServeMux {
	mux := http.NewServeMux()
	NewMatchmakingHandler(queue, profile, testSecret).Register(mux)
	return mux
}

// ── POST /v1/matchmaking/join ─────────────────────────────────────────────────

func TestMatchmakingJoin_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader())
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", "", `{"map_id":"alpha"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMatchmakingJoin_MissingMapID_Returns400(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader(uid))
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", tok, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestMatchmakingJoin_ProfileNotFound_Returns404(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	// profile reader has no profile for uid
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader())
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", tok, `{"map_id":"alpha"}`)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMatchmakingJoin_AlreadyQueued_Returns409(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	queue := &fakeMatchQueue{joinErr: game.ErrAlreadyQueued}
	mux := newMatchmakingMux(queue, newFakeProfileReader(uid))
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", tok, `{"map_id":"alpha"}`)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestMatchmakingJoin_Success_Returns200(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	queue := &fakeMatchQueue{}
	mux := newMatchmakingMux(queue, newFakeProfileReader(uid))
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", tok, `{"map_id":"alpha"}`)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if len(queue.joined) != 1 || queue.joined[0] != uid {
		t.Errorf("queue.joined = %v, want [%s]", queue.joined, uid)
	}
}

func TestMatchmakingJoin_InvalidBody_Returns400(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader(uid))
	w := doRequest(mux, http.MethodPost, "/v1/matchmaking/join", tok, `not json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ── DELETE /v1/matchmaking/leave ──────────────────────────────────────────────

func TestMatchmakingLeave_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader())
	w := doRequest(mux, http.MethodDelete, "/v1/matchmaking/leave", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMatchmakingLeave_Success_Returns204(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	queue := &fakeMatchQueue{}
	mux := newMatchmakingMux(queue, newFakeProfileReader(uid))
	w := doRequest(mux, http.MethodDelete, "/v1/matchmaking/leave", tok, "")
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if len(queue.left) != 1 || queue.left[0] != uid {
		t.Errorf("queue.left = %v, want [%s]", queue.left, uid)
	}
}

// TestMatchmakingLeave_NotQueued_Returns204 verifies that leaving when not
// queued is idempotent: the handler always returns 204.
func TestMatchmakingLeave_NotQueued_Returns204(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	tok := signedToken(t, uid)
	// Leave is fire-and-forget; even with an "error" the handler returns 204.
	mux := newMatchmakingMux(&fakeMatchQueue{}, newFakeProfileReader(uid))
	for i := range 2 {
		w := doRequest(mux, http.MethodDelete, "/v1/matchmaking/leave", tok, "")
		if w.Code != http.StatusNoContent {
			t.Errorf("attempt %d: status = %d, want 204", i+1, w.Code)
		}
	}
}
