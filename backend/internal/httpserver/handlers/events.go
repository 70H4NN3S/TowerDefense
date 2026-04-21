package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/events"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// EventSvc is the domain interface consumed by EventHandler.
type EventSvc interface {
	ActiveAndUpcoming(ctx context.Context) ([]events.Event, error)
	ClaimReward(ctx context.Context, userID, eventID uuid.UUID, tierIndex int) (map[string]int64, error)
}

// EventHandler wires event-related routes onto a ServeMux.
type EventHandler struct {
	svc    EventSvc
	jwtKey []byte
}

// NewEventHandler constructs an EventHandler.
func NewEventHandler(svc EventSvc, jwtKey []byte) *EventHandler {
	return &EventHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *EventHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)
	mux.Handle("GET /v1/events", auth(http.HandlerFunc(h.handleList)))
	mux.Handle("POST /v1/events/{id}/claim", auth(http.HandlerFunc(h.handleClaim)))
}

// ── response shapes ───────────────────────────────────────────────────────────

type eventResponse struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	StartsAt    string          `json:"starts_at"`
	EndsAt      string          `json:"ends_at"`
	Config      json.RawMessage `json:"config"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

// handleList serves GET /v1/events.
// Returns events that are currently active or start within the next 7 days.
func (h *EventHandler) handleList(w http.ResponseWriter, r *http.Request) {
	evs, err := h.svc.ActiveAndUpcoming(r.Context())
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]eventResponse, len(evs))
	for i, ev := range evs {
		out[i] = eventResponse{
			ID:          ev.ID.String(),
			Kind:        ev.Kind,
			Name:        ev.Name,
			Description: ev.Description,
			StartsAt:    ev.StartsAt.UTC().Format("2006-01-02T15:04:05Z"),
			EndsAt:      ev.EndsAt.UTC().Format("2006-01-02T15:04:05Z"),
			Config:      ev.Config,
		}
	}

	respond.JSON(w, http.StatusOK, map[string]any{"events": out})
}

// handleClaim serves POST /v1/events/{id}/claim.
// Body: {"tier": <int>} — zero-based tier index.
func (h *EventHandler) handleClaim(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.JSON(w, http.StatusUnauthorized, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "unauthorized", Message: "Authentication required."},
		})
		return
	}

	idStr := r.PathValue("id")
	eventID, err := uuid.Parse(idStr)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_param", Message: "id must be a valid UUID."},
		})
		return
	}

	var body struct {
		Tier *int `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Tier == nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body must include tier (integer)."},
		})
		return
	}

	rewards, err := h.svc.ClaimReward(r.Context(), userID, eventID, *body.Tier)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"rewards": rewards})
}
