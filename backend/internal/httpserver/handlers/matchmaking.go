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

// MatchQueueSvc is the subset of game.Matchmaker consumed by MatchmakingHandler.
type MatchQueueSvc interface {
	Join(ctx context.Context, userID uuid.UUID, trophies int64, mapID string) error
	Leave(ctx context.Context, userID uuid.UUID)
}

// ProfileReader is the minimal profile interface needed to look up trophies
// before enqueueing a player.
type ProfileReader interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (game.Profile, error)
}

// MatchmakingHandler wires matchmaking routes onto a ServeMux.
type MatchmakingHandler struct {
	queue   MatchQueueSvc
	profile ProfileReader
	jwtKey  []byte
}

// NewMatchmakingHandler constructs a MatchmakingHandler.
func NewMatchmakingHandler(queue MatchQueueSvc, profile ProfileReader, jwtKey []byte) *MatchmakingHandler {
	return &MatchmakingHandler{queue: queue, profile: profile, jwtKey: jwtKey}
}

// Register wires routes onto mux.
func (h *MatchmakingHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)
	mux.Handle("POST /v1/matchmaking/join", auth(http.HandlerFunc(h.handleJoin)))
	mux.Handle("DELETE /v1/matchmaking/leave", auth(http.HandlerFunc(h.handleLeave)))
}

// ── POST /v1/matchmaking/join ─────────────────────────────────────────────────

type joinQueueRequest struct {
	MapID string `json:"map_id"`
}

func (h *MatchmakingHandler) handleJoin(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	var req joinQueueRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
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

	profile, err := h.profile.GetProfile(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	if err := h.queue.Join(r.Context(), userID, profile.Trophies, req.MapID); err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// ── DELETE /v1/matchmaking/leave ──────────────────────────────────────────────

func (h *MatchmakingHandler) handleLeave(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	h.queue.Leave(r.Context(), userID)
	w.WriteHeader(http.StatusNoContent)
}
