package handlers

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/game"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// ── fakeMatchSvc ──────────────────────────────────────────────────────────────

type fakeMatchSvc struct {
	startErr  error
	submitErr error
	match     game.Match
	result    game.MatchResult
}

func newFakeMatchSvc() *fakeMatchSvc {
	id := uuid.New()
	now := time.Now()
	m := game.Match{
		ID:        id,
		PlayerOne: uuid.New(),
		Mode:      "solo",
		MapID:     "alpha",
		Seed:      42,
		StartedAt: now,
		CreatedAt: now,
	}
	return &fakeMatchSvc{
		match: m,
		result: game.MatchResult{
			Match:       m,
			GoldAwarded: 150,
			TrophyDelta: 25,
		},
	}
}

func (f *fakeMatchSvc) StartSinglePlayer(_ context.Context, userID uuid.UUID, mapID string) (game.Match, error) {
	if f.startErr != nil {
		return game.Match{}, f.startErr
	}
	m := f.match
	m.PlayerOne = userID
	m.MapID = mapID
	return m, nil
}

func (f *fakeMatchSvc) SubmitResult(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ game.MatchSummary) (game.MatchResult, error) {
	if f.submitErr != nil {
		return game.MatchResult{}, f.submitErr
	}
	return f.result, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newMatchMux(svc *fakeMatchSvc) *http.ServeMux {
	mux := http.NewServeMux()
	NewMatchHandler(svc, testSecret).Register(mux)
	return mux
}

// ── POST /v1/matches ──────────────────────────────────────────────────────────

func TestMatchStart_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newMatchMux(newFakeMatchSvc())
	w := doRequest(mux, http.MethodPost, "/v1/matches", "", `{"map_id":"alpha"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestMatchStart_MissingBody_Returns400(t *testing.T) {
	t.Parallel()
	mux := newMatchMux(newFakeMatchSvc())
	tok := signedToken(t, uuid.New())
	// Empty map_id.
	w := doRequest(mux, http.MethodPost, "/v1/matches", tok, `{"map_id":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestMatchStart_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()
	mux := newMatchMux(newFakeMatchSvc())
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/matches", tok, `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestMatchStart_HappyPath_Returns201(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	mux := newMatchMux(svc)
	userID := uuid.New()
	tok := signedToken(t, userID)

	w := doRequest(mux, http.MethodPost, "/v1/matches", tok, `{"map_id":"alpha"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	type resp struct {
		Match matchResponse `json:"match"`
	}
	r := decodeResponse[resp](t, w)
	if r.Match.Mode != "solo" {
		t.Errorf("mode = %q, want solo", r.Match.Mode)
	}
	if r.Match.MapID != "alpha" {
		t.Errorf("map_id = %q, want alpha", r.Match.MapID)
	}
}

func TestMatchStart_UnknownMap_Returns400(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	svc.startErr = game.ErrUnknownMap
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/matches", tok, `{"map_id":"nope"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	type errResp struct {
		Error struct{ Code string } `json:"error"`
	}
	r := decodeResponse[errResp](t, w)
	if r.Error.Code != "unknown_map" {
		t.Errorf("error code = %q, want unknown_map", r.Error.Code)
	}
}

func TestMatchStart_InsufficientEnergy_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	svc.startErr = game.ErrInsufficientEnergy
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())

	w := doRequest(mux, http.MethodPost, "/v1/matches", tok, `{"map_id":"alpha"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

// ── POST /v1/matches/{id}/result ──────────────────────────────────────────────

func TestMatchSubmitResult_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newMatchMux(newFakeMatchSvc())
	matchID := uuid.New().String()
	w := doRequest(mux, http.MethodPost, "/v1/matches/"+matchID+"/result", "", `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestMatchSubmitResult_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newMatchMux(newFakeMatchSvc())
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/matches/not-a-uuid/result", tok, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestMatchSubmitResult_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())
	matchID := uuid.New().String()
	body := `{"monsters_killed":10,"waves_cleared":3,"gate_hp":5,"victory":true,"gold_earned":200}`

	w := doRequest(mux, http.MethodPost, "/v1/matches/"+matchID+"/result", tok, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	type resp struct {
		GoldAwarded int64 `json:"gold_awarded"`
		TrophyDelta int64 `json:"trophy_delta"`
	}
	r := decodeResponse[resp](t, w)
	if r.GoldAwarded != 150 {
		t.Errorf("gold_awarded = %d, want 150", r.GoldAwarded)
	}
	if r.TrophyDelta != 25 {
		t.Errorf("trophy_delta = %d, want 25", r.TrophyDelta)
	}
}

func TestMatchSubmitResult_MatchNotFound_Returns404(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	svc.submitErr = game.ErrMatchNotFound
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())
	matchID := uuid.New().String()

	w := doRequest(mux, http.MethodPost, "/v1/matches/"+matchID+"/result", tok, `{}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestMatchSubmitResult_NotOwned_Returns403(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	svc.submitErr = game.ErrMatchNotOwned
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())
	matchID := uuid.New().String()

	w := doRequest(mux, http.MethodPost, "/v1/matches/"+matchID+"/result", tok, `{}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestMatchSubmitResult_AlreadyEnded_Returns409(t *testing.T) {
	t.Parallel()
	svc := newFakeMatchSvc()
	svc.submitErr = game.ErrMatchAlreadyEnded
	mux := newMatchMux(svc)
	tok := signedToken(t, uuid.New())
	matchID := uuid.New().String()

	w := doRequest(mux, http.MethodPost, "/v1/matches/"+matchID+"/result", tok, `{}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}
