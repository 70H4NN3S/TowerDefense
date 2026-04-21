package handlers

import (
	"context"
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ShopService is the domain interface consumed by ShopHandler.
type ShopService interface {
	ListCatalogForUser(ctx context.Context, userID uuid.UUID) ([]game.CatalogEntry, error)
	Purchase(ctx context.Context, userID, templateID uuid.UUID) (game.OwnedTower, error)
}

// ShopHandler wires shop-related routes onto a ServeMux.
type ShopHandler struct {
	svc    ShopService
	jwtKey []byte
}

// NewShopHandler constructs a ShopHandler.
func NewShopHandler(svc ShopService, jwtKey []byte) *ShopHandler {
	return &ShopHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *ShopHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)

	mux.Handle("GET /v1/shop/towers", auth(http.HandlerFunc(h.handleListCatalog)))
	mux.Handle("POST /v1/shop/towers/{id}/buy", auth(http.HandlerFunc(h.handleBuy)))
}

// catalogEntryResponse is the JSON shape for a single shop entry.
type catalogEntryResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Rarity       string `json:"rarity"`
	BaseDamage   int64  `json:"base_damage"`
	BaseRange    int64  `json:"base_range"`
	BaseRate     int64  `json:"base_rate"`
	CostDiamonds int64  `json:"cost_diamonds"`
	Description  string `json:"description"`
	Owned        bool   `json:"owned"`
}

func catalogEntryToResponse(e game.CatalogEntry) catalogEntryResponse {
	return catalogEntryResponse{
		ID:           e.Template.ID.String(),
		Name:         e.Template.Name,
		Rarity:       e.Template.Rarity,
		BaseDamage:   e.Template.BaseDamage,
		BaseRange:    e.Template.BaseRange,
		BaseRate:     e.Template.BaseRate,
		CostDiamonds: e.Template.CostDiamonds,
		Description:  e.Template.Description,
		Owned:        e.Owned,
	}
}

// handleListCatalog serves GET /v1/shop/towers.
func (h *ShopHandler) handleListCatalog(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	entries, err := h.svc.ListCatalogForUser(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]catalogEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = catalogEntryToResponse(e)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"towers": out})
}

// handleBuy serves POST /v1/shop/towers/{id}/buy.
func (h *ShopHandler) handleBuy(w http.ResponseWriter, r *http.Request) {
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

	tower, err := h.svc.Purchase(r.Context(), userID, templateID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"tower": ownedTowerToResponse(tower)})
}
