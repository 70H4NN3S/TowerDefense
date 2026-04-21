package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// MatchSvc is the domain interface consumed by MatchHandler.
type MatchSvc interface {
	StartSinglePlayer(ctx context.Context, userID uuid.UUID, mapID string) (game.Match, error)
	SubmitResult(ctx context.Context, userID, matchID uuid.UUID, summary game.MatchSummary) (game.MatchResult, error)
}

// MatchHandler wires match-related routes onto a ServeMux.
type MatchHandler struct {
	svc    MatchSvc
	jwtKey []byte
}

// NewMatchHandler constructs a MatchHandler.
func NewMatchHandler(svc MatchSvc, jwtKey []byte) *MatchHandler {
	return &MatchHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *MatchHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)
	mux.Handle("POST /v1/matches", auth(http.HandlerFunc(h.handleStart)))
	mux.Handle("POST /v1/matches/{id}/result", auth(http.HandlerFunc(h.handleSubmitResult)))
}

// ── response shapes ───────────────────────────────────────────────────────────

type matchResponse struct {
	ID        string  `json:"id"`
	PlayerOne string  `json:"player_one"`
	Mode      string  `json:"mode"`
	MapID     string  `json:"map_id"`
	Seed      int64   `json:"seed"`
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at,omitempty"`
	Winner    *string `json:"winner,omitempty"`
}

func matchToResponse(m game.Match) matchResponse {
	r := matchResponse{
		ID:        m.ID.String(),
		PlayerOne: m.PlayerOne.String(),
		Mode:      m.Mode,
		MapID:     m.MapID,
		Seed:      m.Seed,
		StartedAt: m.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if m.EndedAt != nil {
		s := m.EndedAt.Format("2006-01-02T15:04:05Z07:00")
		r.EndedAt = &s
	}
	if m.Winner != nil {
		s := m.Winner.String()
		r.Winner = &s
	}
	return r
}

// ── POST /v1/matches ──────────────────────────────────────────────────────────

type startMatchRequest struct {
	MapID string `json:"map_id"`
}

func (h *MatchHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	var req startMatchRequest
	if err := decodeBody(r, &req); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is not valid JSON."},
		})
		return
	}
	if req.MapID == "" {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "validation_failed", Message: "map_id is required."},
		})
		return
	}

	m, err := h.svc.StartSinglePlayer(r.Context(), userID, req.MapID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"match": matchToResponse(m)})
}

// ── POST /v1/matches/{id}/result ──────────────────────────────────────────────

func (h *MatchHandler) handleSubmitResult(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	matchIDStr := r.PathValue("id")
	matchID, err := uuid.Parse(matchIDStr)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "Match ID is not a valid UUID."},
		})
		return
	}

	var summary game.MatchSummary
	if err := json.NewDecoder(r.Body).Decode(&summary); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is not valid JSON."},
		})
		return
	}

	result, err := h.svc.SubmitResult(r.Context(), userID, matchID, summary)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"match":        matchToResponse(result.Match),
		"gold_awarded": result.GoldAwarded,
		"trophy_delta": result.TrophyDelta,
	})
}
