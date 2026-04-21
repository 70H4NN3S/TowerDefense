package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/alliance"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// AllianceSvc is the domain interface consumed by AllianceHandler.
type AllianceSvc interface {
	Create(ctx context.Context, leaderID uuid.UUID, name, tag, description string) (alliance.Alliance, error)
	Get(ctx context.Context, allianceID uuid.UUID) (alliance.Alliance, error)
	GetMembership(ctx context.Context, userID uuid.UUID) (alliance.Member, error)
	ListMembers(ctx context.Context, allianceID uuid.UUID) ([]alliance.Member, error)
	Disband(ctx context.Context, requesterID, allianceID uuid.UUID) error
	Invite(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) (alliance.Invite, error)
	AcceptInvite(ctx context.Context, userID, inviteID uuid.UUID) error
	DeclineInvite(ctx context.Context, userID, inviteID uuid.UUID) error
	Leave(ctx context.Context, userID uuid.UUID) error
	Promote(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error
	Demote(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error
	Kick(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error
}

// AllianceHandler wires alliance-related routes onto a ServeMux.
type AllianceHandler struct {
	svc    AllianceSvc
	jwtKey []byte
}

// NewAllianceHandler constructs an AllianceHandler.
func NewAllianceHandler(svc AllianceSvc, jwtKey []byte) *AllianceHandler {
	return &AllianceHandler{svc: svc, jwtKey: jwtKey}
}

// Register wires routes onto mux. All routes require authentication.
func (h *AllianceHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)

	// Alliance CRUD
	mux.Handle("POST /v1/alliances", auth(http.HandlerFunc(h.handleCreate)))
	mux.Handle("GET /v1/alliances/{id}", auth(http.HandlerFunc(h.handleGet)))
	mux.Handle("DELETE /v1/alliances/{id}", auth(http.HandlerFunc(h.handleDisband)))

	// Member management
	mux.Handle("GET /v1/alliances/{id}/members", auth(http.HandlerFunc(h.handleListMembers)))
	mux.Handle("DELETE /v1/alliances/{id}/members/{userID}", auth(http.HandlerFunc(h.handleKick)))
	mux.Handle("POST /v1/alliances/{id}/members/{userID}/promote", auth(http.HandlerFunc(h.handlePromote)))
	mux.Handle("POST /v1/alliances/{id}/members/{userID}/demote", auth(http.HandlerFunc(h.handleDemote)))

	// Invites
	mux.Handle("POST /v1/alliances/{id}/invites", auth(http.HandlerFunc(h.handleInvite)))
	mux.Handle("POST /v1/invites/{id}/accept", auth(http.HandlerFunc(h.handleAcceptInvite)))
	mux.Handle("POST /v1/invites/{id}/decline", auth(http.HandlerFunc(h.handleDeclineInvite)))

	// Self-service
	mux.Handle("GET /v1/me/alliance", auth(http.HandlerFunc(h.handleGetMembership)))
	mux.Handle("POST /v1/me/alliance/leave", auth(http.HandlerFunc(h.handleLeave)))
}

// ── response shapes ───────────────────────────────────────────────────────────

type allianceResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Tag         string  `json:"tag"`
	Description string  `json:"description"`
	LeaderID    string  `json:"leader_id"`
	ChannelID   *string `json:"channel_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func allianceToResponse(a alliance.Alliance) allianceResponse {
	r := allianceResponse{
		ID:          a.ID.String(),
		Name:        a.Name,
		Tag:         a.Tag,
		Description: a.Description,
		LeaderID:    a.LeaderID.String(),
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
	}
	if a.ChannelID != nil {
		s := a.ChannelID.String()
		r.ChannelID = &s
	}
	return r
}

type memberResponse struct {
	UserID     string `json:"user_id"`
	AllianceID string `json:"alliance_id"`
	Role       string `json:"role"`
	JoinedAt   string `json:"joined_at"`
}

func memberToResponse(m alliance.Member) memberResponse {
	return memberResponse{
		UserID:     m.UserID.String(),
		AllianceID: m.AllianceID.String(),
		Role:       m.Role,
		JoinedAt:   m.JoinedAt.UTC().Format(time.RFC3339),
	}
}

type inviteResponse struct {
	ID         string `json:"id"`
	AllianceID string `json:"alliance_id"`
	UserID     string `json:"user_id"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func inviteToResponse(inv alliance.Invite) inviteResponse {
	return inviteResponse{
		ID:         inv.ID.String(),
		AllianceID: inv.AllianceID.String(),
		UserID:     inv.UserID.String(),
		Status:     inv.Status,
		CreatedAt:  inv.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// handleCreate serves POST /v1/alliances.
// JSON body: {"name": "...", "tag": "...", "description": "..."}
func (h *AllianceHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotInAlliance)
		return
	}

	var body struct {
		Name        string `json:"name"`
		Tag         string `json:"tag"`
		Description string `json:"description"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is malformed."},
		})
		return
	}

	a, err := h.svc.Create(r.Context(), userID, body.Name, body.Tag, body.Description)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"alliance": allianceToResponse(a)})
}

// handleGet serves GET /v1/alliances/{id}.
func (h *AllianceHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	a, err := h.svc.Get(r.Context(), allianceID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"alliance": allianceToResponse(a)})
}

// handleDisband serves DELETE /v1/alliances/{id}.
func (h *AllianceHandler) handleDisband(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotMember)
		return
	}

	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Disband(r.Context(), userID, allianceID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListMembers serves GET /v1/alliances/{id}/members.
func (h *AllianceHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	members, err := h.svc.ListMembers(r.Context(), allianceID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]memberResponse, len(members))
	for i, m := range members {
		out[i] = memberToResponse(m)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"members": out})
}

// handleKick serves DELETE /v1/alliances/{id}/members/{userID}.
func (h *AllianceHandler) handleKick(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotMember)
		return
	}

	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "User ID is not a valid UUID."},
		})
		return
	}

	if err := h.svc.Kick(r.Context(), requesterID, allianceID, targetID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePromote serves POST /v1/alliances/{id}/members/{userID}/promote.
func (h *AllianceHandler) handlePromote(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotMember)
		return
	}

	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "User ID is not a valid UUID."},
		})
		return
	}

	if err := h.svc.Promote(r.Context(), requesterID, allianceID, targetID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDemote serves POST /v1/alliances/{id}/members/{userID}/demote.
func (h *AllianceHandler) handleDemote(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotMember)
		return
	}

	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	targetID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "User ID is not a valid UUID."},
		})
		return
	}

	if err := h.svc.Demote(r.Context(), requesterID, allianceID, targetID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleInvite serves POST /v1/alliances/{id}/invites.
// JSON body: {"user_id": "..."}
func (h *AllianceHandler) handleInvite(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotMember)
		return
	}

	allianceID, ok := parseAllianceID(w, r)
	if !ok {
		return
	}

	var body struct {
		UserID string `json:"user_id"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is malformed."},
		})
		return
	}

	targetID, err := uuid.Parse(body.UserID)
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "user_id is not a valid UUID."},
		})
		return
	}

	inv, err := h.svc.Invite(r.Context(), requesterID, allianceID, targetID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"invite": inviteToResponse(inv)})
}

// handleAcceptInvite serves POST /v1/invites/{id}/accept.
func (h *AllianceHandler) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrInviteNotFound)
		return
	}

	inviteID, ok := parseInviteID(w, r)
	if !ok {
		return
	}

	if err := h.svc.AcceptInvite(r.Context(), userID, inviteID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeclineInvite serves POST /v1/invites/{id}/decline.
func (h *AllianceHandler) handleDeclineInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrInviteNotFound)
		return
	}

	inviteID, ok := parseInviteID(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeclineInvite(r.Context(), userID, inviteID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetMembership serves GET /v1/me/alliance.
func (h *AllianceHandler) handleGetMembership(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotInAlliance)
		return
	}

	m, err := h.svc.GetMembership(r.Context(), userID)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"membership": memberToResponse(m)})
}

// handleLeave serves POST /v1/me/alliance/leave.
func (h *AllianceHandler) handleLeave(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, alliance.ErrNotInAlliance)
		return
	}

	if err := h.svc.Leave(r.Context(), userID); err != nil {
		respond.Error(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── shared helpers ─────────────────────────────────────────────────────────────

func parseAllianceID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "Alliance ID is not a valid UUID."},
		})
		return "", false
	}
	return id, true
}

func parseInviteID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "Invite ID is not a valid UUID."},
		})
		return "", false
	}
	return id, true
}
