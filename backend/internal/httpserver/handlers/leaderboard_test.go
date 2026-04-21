package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/leaderboard"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fake leaderboard service ──────────────────────────────────────────────────

type fakeLeaderboardSvc struct {
	globalEntries   []leaderboard.GlobalEntry
	allianceEntries []leaderboard.AllianceEntry
	memberEntries   []leaderboard.MemberEntry
	globalErr       error
	allianceErr     error
	memberErr       error
}

func (f *fakeLeaderboardSvc) GlobalLeaderboard(_ context.Context, afterRank int64, limit int) ([]leaderboard.GlobalEntry, error) {
	if f.globalErr != nil {
		return nil, f.globalErr
	}
	var out []leaderboard.GlobalEntry
	for _, e := range f.globalEntries {
		if e.Rank > afterRank {
			out = append(out, e)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeLeaderboardSvc) AllianceLeaderboard(_ context.Context, _ int64, _ uuid.UUID, limit int, _ bool) ([]leaderboard.AllianceEntry, error) {
	if f.allianceErr != nil {
		return nil, f.allianceErr
	}
	out := f.allianceEntries
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeLeaderboardSvc) AllianceMemberLeaderboard(_ context.Context, _ uuid.UUID) ([]leaderboard.MemberEntry, error) {
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	return f.memberEntries, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newLeaderboardMux(svc *fakeLeaderboardSvc) *http.ServeMux {
	mux := http.NewServeMux()
	NewLeaderboardHandler(svc, testSecret).Register(mux)
	return mux
}

func globalPath() string                { return "/v1/leaderboard/global" }
func allianceLBPath() string            { return "/v1/leaderboard/alliances" }
func allianceMemberLBPath(id uuid.UUID) string {
	return fmt.Sprintf("/v1/alliances/%s/leaderboard", id)
}

// ── GET /v1/leaderboard/global ────────────────────────────────────────────────

func TestLeaderboardGlobal_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	w := doRequest(mux, http.MethodGet, globalPath(), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestLeaderboardGlobal_EmptyView_Returns200WithEmptySlice(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, globalPath(), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["entries"]; !ok {
		t.Error("response missing 'entries' key")
	}
}

func TestLeaderboardGlobal_WithEntries_ReturnsRankedList(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		globalEntries: []leaderboard.GlobalEntry{
			{Rank: 1, UserID: uuid.New(), Trophies: 1000},
			{Rank: 2, UserID: uuid.New(), Trophies: 900},
		},
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, globalPath(), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Entries []struct {
			Rank     float64 `json:"rank"`
			Trophies float64 `json:"trophies"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(resp.Entries))
	}
	if resp.Entries[0].Rank != 1 {
		t.Errorf("first rank = %v, want 1", resp.Entries[0].Rank)
	}
}

func TestLeaderboardGlobal_CursorParam_FiltersCorrectly(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		globalEntries: []leaderboard.GlobalEntry{
			{Rank: 1, UserID: uuid.New(), Trophies: 1000},
			{Rank: 2, UserID: uuid.New(), Trophies: 900},
			{Rank: 3, UserID: uuid.New(), Trophies: 800},
		},
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	// Request page starting after rank 1.
	w := doRequest(mux, http.MethodGet, globalPath()+"?after_rank=1&limit=2", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Entries []struct{ Rank float64 `json:"rank"` } `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("len = %d, want 2", len(resp.Entries))
	}
	if resp.Entries[0].Rank != 2 {
		t.Errorf("first rank = %v, want 2", resp.Entries[0].Rank)
	}
}

func TestLeaderboardGlobal_NextCursorPresentWhenFullPage(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		globalEntries: func() []leaderboard.GlobalEntry {
			out := make([]leaderboard.GlobalEntry, 5)
			for i := range 5 {
				out[i] = leaderboard.GlobalEntry{Rank: int64(i + 1), UserID: uuid.New()}
			}
			return out
		}(),
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, globalPath()+"?limit=3", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["next_cursor"] == nil {
		t.Error("next_cursor should be present when a full page is returned")
	}
}

func TestLeaderboardGlobal_NextCursorAbsentOnLastPage(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		globalEntries: []leaderboard.GlobalEntry{
			{Rank: 1, UserID: uuid.New()},
			{Rank: 2, UserID: uuid.New()},
		},
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	// Request limit=10 but only 2 entries exist → last page.
	w := doRequest(mux, http.MethodGet, globalPath()+"?limit=10", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["next_cursor"] != nil {
		t.Errorf("next_cursor = %v, want nil on last page", resp["next_cursor"])
	}
}

func TestLeaderboardGlobal_InvalidAfterRank_Returns400(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, globalPath()+"?after_rank=notanumber", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLeaderboardGlobal_InvalidLimit_Returns400(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, globalPath()+"?limit=0", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ── GET /v1/leaderboard/alliances ────────────────────────────────────────────

func TestLeaderboardAlliances_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	w := doRequest(mux, http.MethodGet, allianceLBPath(), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestLeaderboardAlliances_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		allianceEntries: []leaderboard.AllianceEntry{
			{AllianceID: uuid.New(), AllianceName: "Alpha", AllianceTag: "ALP", TotalTrophies: 5000, MemberCount: 10},
		},
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, allianceLBPath(), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	entries, ok := resp["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Errorf("entries = %v, want array of 1", resp["entries"])
	}
}

func TestLeaderboardAlliances_InvalidCursorParams_Returns400(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())

	tests := []struct{ name, query string }{
		{"bad after_trophies", "after_trophies=abc&after_id=" + uuid.New().String()},
		{"bad after_id", "after_trophies=100&after_id=not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := doRequest(mux, http.MethodGet, allianceLBPath()+"?"+tt.query, tok, "")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

// ── GET /v1/alliances/{id}/leaderboard ───────────────────────────────────────

func TestLeaderboardAllianceMembers_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	w := doRequest(mux, http.MethodGet, allianceMemberLBPath(uuid.New()), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestLeaderboardAllianceMembers_InvalidAllianceUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/alliances/not-a-uuid/leaderboard", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLeaderboardAllianceMembers_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := &fakeLeaderboardSvc{
		memberEntries: []leaderboard.MemberEntry{
			{Rank: 1, UserID: uuid.New(), Role: "leader", Trophies: 1000},
			{Rank: 2, UserID: uuid.New(), Role: "member", Trophies: 500},
		},
	}
	mux := newLeaderboardMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, allianceMemberLBPath(uuid.New()), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	entries, ok := resp["entries"].([]any)
	if !ok || len(entries) != 2 {
		t.Errorf("entries = %v, want array of 2", resp["entries"])
	}
}

func TestLeaderboardAllianceMembers_EmptyAlliance_Returns200EmptySlice(t *testing.T) {
	t.Parallel()
	mux := newLeaderboardMux(&fakeLeaderboardSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, allianceMemberLBPath(uuid.New()), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Entries []any `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("entries len = %d, want 0", len(resp.Entries))
	}
}
