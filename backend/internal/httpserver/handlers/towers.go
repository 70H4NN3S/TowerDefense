package handlers

import (
	"context"
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// TowerService is the domain interface consumed by TowerHandler.
type TowerService interface {
	ListOwned(ctx context.Context, userID uuid.UUID) ([]game.OwnedTower, error)
	LevelUp(ctx context.Context, userID, templateID uuid.UUID) (game.OwnedTower, error)
}

// TowerHandler wires tower-ownership routes onto a ServeMux.
type TowerHandler struct {
	svc    TowerService
	jwtKey []byte
}

// NewTowerHandler constructs a TowerHandler.
func NewTowerHandler(svc TowerService, jwtKey []byte) *TowerHandler {
	return &TowerHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *TowerHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)

	mux.Handle("GET /v1/towers", auth(http.HandlerFunc(h.handleListOwned)))
	mux.Handle("POST /v1/towers/{id}/upgrade", auth(http.HandlerFunc(h.handleUpgrade)))
}

// towerLevelResponse describes a single level's stats.
type towerLevelResponse struct {
	Level    int   `json:"level"`
	GoldCost int64 `json:"gold_cost"`
	Damage   int64 `json:"damage"`
	Range    int64 `json:"range"`
	Rate     int64 `json:"rate"`
}

// ownedTowerResponse is the JSON shape for an owned tower.
type ownedTowerResponse struct {
	TemplateID   string             `json:"template_id"`
	Name         string             `json:"name"`
	Rarity       string             `json:"rarity"`
	CostDiamonds int64              `json:"cost_diamonds"`
	Description  string             `json:"description"`
	Current      towerLevelResponse `json:"current"`
}

func ownedTowerToResponse(o game.OwnedTower) ownedTowerResponse {
	return ownedTowerResponse{
		TemplateID:   o.TemplateID.String(),
		Name:         o.Template.Name,
		Rarity:       o.Template.Rarity,
		CostDiamonds: o.Template.CostDiamonds,
		Description:  o.Template.Description,
		Current: towerLevelResponse{
			Level:    o.Level,
			GoldCost: o.Current.GoldCost,
			Damage:   o.Current.Damage,
			Range:    o.Current.Range,
			Rate:     o.Current.Rate,
		},
	}
}

// handleListOwned serves GET /v1/towers.
func (h *TowerHandler) handleListOwned(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	towers, err := h.svc.ListOwned(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]ownedTowerResponse, len(towers))
	for i, t := range towers {
		out[i] = ownedTowerToResponse(t)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"towers": out})
}

// handleUpgrade serves POST /v1/towers/{id}/upgrade.
func (h *TowerHandler) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	templateIDStr := r.PathValue("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "Tower ID is not a valid UUID."},
		})
		return
	}

	tower, err := h.svc.LevelUp(r.Context(), userID, templateID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"tower": ownedTowerToResponse(tower)})
}
