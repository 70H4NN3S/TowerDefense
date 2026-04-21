package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/leaderboard"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// LeaderboardSvc is the domain interface consumed by LeaderboardHandler.
type LeaderboardSvc interface {
	GlobalLeaderboard(ctx context.Context, afterRank int64, limit int) ([]leaderboard.GlobalEntry, error)
	AllianceLeaderboard(ctx context.Context, afterTrophies int64, afterAllianceID uuid.UUID, limit int, firstPage bool) ([]leaderboard.AllianceEntry, error)
	AllianceMemberLeaderboard(ctx context.Context, allianceID uuid.UUID) ([]leaderboard.MemberEntry, error)
}

// LeaderboardHandler wires leaderboard-related routes onto a ServeMux.
type LeaderboardHandler struct {
	svc    LeaderboardSvc
	jwtKey []byte
}

// NewLeaderboardHandler constructs a LeaderboardHandler.
func NewLeaderboardHandler(svc LeaderboardSvc, jwtKey []byte) *LeaderboardHandler {
	return &LeaderboardHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *LeaderboardHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)
	mux.Handle("GET /v1/leaderboard/global", auth(http.HandlerFunc(h.handleGlobal)))
	mux.Handle("GET /v1/leaderboard/alliances", auth(http.HandlerFunc(h.handleAlliances)))
	mux.Handle("GET /v1/alliances/{id}/leaderboard", auth(http.HandlerFunc(h.handleAllianceMembers)))
}

// ── response shapes ───────────────────────────────────────────────────────────

type globalEntryResponse struct {
	Rank     int64  `json:"rank"`
	UserID   string `json:"user_id"`
	Trophies int64  `json:"trophies"`
}

type allianceEntryResponse struct {
	AllianceID    string `json:"alliance_id"`
	AllianceName  string `json:"alliance_name"`
	AllianceTag   string `json:"alliance_tag"`
	TotalTrophies int64  `json:"total_trophies"`
	MemberCount   int64  `json:"member_count"`
}

type memberEntryResponse struct {
	Rank     int64  `json:"rank"`
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	Trophies int64  `json:"trophies"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

// handleGlobal serves GET /v1/leaderboard/global.
// Query params:
//   - after_rank  int  exclusive rank cursor (default 0 = first page)
//   - limit       int  1–100 (default 25)
func (h *LeaderboardHandler) handleGlobal(w http.ResponseWriter, r *http.Request) {
	afterRank, ok := parseIntParam(w, r, "after_rank", 0, 0, 1<<62)
	if !ok {
		return
	}
	limit, ok := parseIntParam(w, r, "limit", 25, 1, 100)
	if !ok {
		return
	}

	entries, err := h.svc.GlobalLeaderboard(r.Context(), afterRank, int(limit))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]globalEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = globalEntryResponse{
			Rank:     e.Rank,
			UserID:   e.UserID.String(),
			Trophies: e.Trophies,
		}
	}

	var nextCursor *int64
	if len(entries) == int(limit) {
		c := entries[len(entries)-1].Rank
		nextCursor = &c
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"entries":     out,
		"next_cursor": nextCursor,
	})
}

// handleAlliances serves GET /v1/leaderboard/alliances.
// Query params:
//   - after_trophies  int     composite cursor part 1 (omit for first page)
//   - after_id        string  composite cursor part 2 — alliance UUID
//   - limit           int     1–100 (default 25)
func (h *LeaderboardHandler) handleAlliances(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseIntParam(w, r, "limit", 25, 1, 100)
	if !ok {
		return
	}

	afterTrophiesRaw := r.URL.Query().Get("after_trophies")
	afterIDRaw := r.URL.Query().Get("after_id")
	firstPage := afterTrophiesRaw == "" && afterIDRaw == ""

	var afterTrophies int64
	var afterID uuid.UUID

	if !firstPage {
		t, err := strconv.ParseInt(afterTrophiesRaw, 10, 64)
		if err != nil || t < 0 {
			respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
				Error: respond.ErrorDetail{Code: "invalid_param", Message: "after_trophies must be a non-negative integer."},
			})
			return
		}
		afterTrophies = t

		id, err := uuid.Parse(afterIDRaw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
				Error: respond.ErrorDetail{Code: "invalid_param", Message: "after_id must be a valid UUID."},
			})
			return
		}
		afterID = id
	}

	entries, err := h.svc.AllianceLeaderboard(r.Context(), afterTrophies, afterID, int(limit), firstPage)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]allianceEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = allianceEntryResponse{
			AllianceID:    e.AllianceID.String(),
			AllianceName:  e.AllianceName,
			AllianceTag:   e.AllianceTag,
			TotalTrophies: e.TotalTrophies,
			MemberCount:   e.MemberCount,
		}
	}

	var nextCursorTrophies *int64
	var nextCursorID *string
	if len(entries) == int(limit) {
		last := entries[len(entries)-1]
		nextCursorTrophies = &last.TotalTrophies
		s := last.AllianceID.String()
		nextCursorID = &s
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"entries":              out,
		"next_cursor_trophies": nextCursorTrophies,
		"next_cursor_id":       nextCursorID,
	})
}

// handleAllianceMembers serves GET /v1/alliances/{id}/leaderboard.
func (h *LeaderboardHandler) handleAllianceMembers(w http.ResponseWriter, r *http.Request) {
	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	entries, err := h.svc.AllianceMemberLeaderboard(r.Context(), allianceID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]memberEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = memberEntryResponse{
			Rank:     e.Rank,
			UserID:   e.UserID.String(),
			Role:     e.Role,
			Trophies: e.Trophies,
		}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"entries": out})
}

// ── shared param helpers ──────────────────────────────────────────────────────

// parseIntParam reads a query parameter as int64 with a default and range check.
// It writes a 400 error and returns false on any validation failure.
func parseIntParam(w http.ResponseWriter, r *http.Request, name string, defaultVal, min, max int64) (int64, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return defaultVal, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < min || v > max {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{
				Code:    "invalid_param",
				Message: name + " must be an integer in range [" + strconv.FormatInt(min, 10) + ", " + strconv.FormatInt(max, 10) + "].",
			},
		})
		return 0, false
	}
	return v, true
}
