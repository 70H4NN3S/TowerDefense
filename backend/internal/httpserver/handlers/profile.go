package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/models"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

const (
	maxDisplayNameLen = 32
	maxAvatarID       = 99
)

// ProfileService is the domain interface consumed by ProfileHandler.
// Declared consumer-side so tests can supply a fake.
type ProfileService interface {
	CreateProfile(ctx context.Context, userID uuid.UUID) (game.Profile, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (game.Profile, error)
	UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) (game.Profile, error)
	UpdateAvatarID(ctx context.Context, userID uuid.UUID, avatarID int) (game.Profile, error)
}

// ProfileHandler wires profile-related routes onto a ServeMux.
type ProfileHandler struct {
	svc    ProfileService
	jwtKey []byte
}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler(svc ProfileService, jwtKey []byte) *ProfileHandler {
	return &ProfileHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *ProfileHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)

	mux.Handle("GET /v1/me", auth(http.HandlerFunc(h.handleGetMe)))
	mux.Handle("PATCH /v1/me", auth(http.HandlerFunc(h.handlePatchMe)))
}

// profileResponse is the JSON shape for profile API responses.
type profileResponse struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarID    int    `json:"avatar_id"`
	Trophies    int64  `json:"trophies"`
	Gold        int64  `json:"gold"`
	Diamonds    int64  `json:"diamonds"`
	Energy      int    `json:"energy"`
	EnergyMax   int    `json:"energy_max"`
	XP          int64  `json:"xp"`
	Level       int    `json:"level"`
}

func profileToResponse(p game.Profile) profileResponse {
	return profileResponse{
		UserID:      p.UserID.String(),
		DisplayName: p.DisplayName,
		AvatarID:    p.AvatarID,
		Trophies:    p.Trophies,
		Gold:        p.Gold,
		Diamonds:    p.Diamonds,
		Energy:      p.Energy,
		EnergyMax:   game.EnergyMax,
		XP:          p.XP,
		Level:       p.Level,
	}
}

// handleGetMe serves GET /v1/me.
// Creates a profile on first call (lazy initialisation) then returns it.
func (h *ProfileHandler) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	p, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		// No profile yet — create one with defaults and return it.
		if isNotFound(err) {
			p, err = h.svc.CreateProfile(r.Context(), userID)
			if err != nil {
				respond.Error(w, r, err)
				return
			}
			respond.JSON(w, http.StatusCreated, profileToResponse(p))
			return
		}
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, profileToResponse(p))
}

// patchMeRequest is the JSON body for PATCH /v1/me.
// Both fields are optional; omitting a field leaves it unchanged.
type patchMeRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarID    *int    `json:"avatar_id"`
}

// handlePatchMe serves PATCH /v1/me.
func (h *ProfileHandler) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, game.ErrProfileNotFound)
		return
	}

	var req patchMeRequest
	if err := decodeBody(r, &req); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is not valid JSON."},
		})
		return
	}

	if err := validatePatchMe(req); err != nil {
		respond.Error(w, r, err)
		return
	}

	p, err := h.svc.GetProfile(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	if req.DisplayName != nil {
		p, err = h.svc.UpdateDisplayName(r.Context(), userID, strings.TrimSpace(*req.DisplayName))
		if err != nil {
			respond.Error(w, r, err)
			return
		}
	}

	if req.AvatarID != nil {
		p, err = h.svc.UpdateAvatarID(r.Context(), userID, *req.AvatarID)
		if err != nil {
			respond.Error(w, r, err)
			return
		}
	}

	respond.JSON(w, http.StatusOK, profileToResponse(p))
}

func validatePatchMe(req patchMeRequest) error {
	v := &models.ValidationError{}

	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if utf8.RuneCountInString(name) > maxDisplayNameLen {
			v.Add("display_name", "must not exceed 32 characters")
		}
	}

	if req.AvatarID != nil {
		if *req.AvatarID < 0 || *req.AvatarID > maxAvatarID {
			v.Add("avatar_id", "must be between 0 and 99")
		}
	}

	if v.HasErrors() {
		return v
	}
	return nil
}

// isNotFound reports whether err wraps game.ErrProfileNotFound.
func isNotFound(err error) bool {
	return errors.Is(err, game.ErrProfileNotFound)
}
