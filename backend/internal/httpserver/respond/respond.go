// Package respond provides JSON response helpers and the standard API error
// envelope. Both the httpserver package and its handlers sub-package import
// this package, so it must not import either of them.
package respond

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/70H4NN3S/TowerDefense/internal/alliance"
	"github.com/70H4NN3S/TowerDefense/internal/auth"
	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/events"
	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/models"
)

// ErrorEnvelope is the standard API error response shape described in
// .claude/rules/error-handling.md.
type ErrorEnvelope struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id,omitempty"`
}

// ErrorDetail carries the machine-readable code and a user-safe message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// JSON sets Content-Type to application/json, writes status, and JSON-
// encodes v into the response body. Encoding errors are logged but not
// surfaced to the caller because the header has already been sent.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode response", "err", err)
	}
}

// Error maps err to an HTTP status and writes a JSON error envelope.
// All domain-specific mappings are centralised here per error-handling.md.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.RequestIDFromContext(r.Context())

	// Validation errors: 400 with per-field details.
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		JSON(w, http.StatusBadRequest, ErrorEnvelope{
			Error:     ErrorDetail{Code: "validation_failed", Message: "One or more fields are invalid.", Details: map[string]any{"fields": ve.Fields}},
			RequestID: reqID,
		})
		return
	}

	switch {
	// Auth errors
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeErr(w, http.StatusUnauthorized, reqID, "invalid_credentials", "Invalid email or password.")
	case errors.Is(err, auth.ErrEmailTaken):
		writeErr(w, http.StatusConflict, reqID, "email_taken", "This email address is already registered.")
	case errors.Is(err, auth.ErrUsernameTaken):
		writeErr(w, http.StatusConflict, reqID, "username_taken", "This username is already taken.")
	case errors.Is(err, auth.ErrNotFound):
		writeErr(w, http.StatusNotFound, reqID, "not_found", "Resource not found.")
	case errors.Is(err, auth.ErrExpiredToken):
		writeErr(w, http.StatusUnauthorized, reqID, "token_expired", "Token has expired.")
	case errors.Is(err, auth.ErrInvalidToken):
		writeErr(w, http.StatusUnauthorized, reqID, "invalid_token", "Token is invalid.")
	// Game errors
	case errors.Is(err, game.ErrProfileNotFound):
		writeErr(w, http.StatusNotFound, reqID, "profile_not_found", "Player profile not found.")
	case errors.Is(err, game.ErrInsufficientGold):
		writeErr(w, http.StatusConflict, reqID, "insufficient_gold", "Not enough gold.")
	case errors.Is(err, game.ErrInsufficientDiamonds):
		writeErr(w, http.StatusConflict, reqID, "insufficient_diamonds", "Not enough diamonds.")
	case errors.Is(err, game.ErrInsufficientEnergy):
		writeErr(w, http.StatusConflict, reqID, "insufficient_energy", "Not enough energy.")
	case errors.Is(err, game.ErrTemplateNotFound):
		writeErr(w, http.StatusNotFound, reqID, "tower_not_found", "Tower template not found.")
	case errors.Is(err, game.ErrAlreadyOwned):
		writeErr(w, http.StatusConflict, reqID, "already_owned", "You already own this tower.")
	case errors.Is(err, game.ErrNotOwned):
		writeErr(w, http.StatusConflict, reqID, "not_owned", "You do not own this tower.")
	case errors.Is(err, game.ErrMaxLevel):
		writeErr(w, http.StatusConflict, reqID, "max_level", "Tower is already at max level.")
	// Match errors
	case errors.Is(err, game.ErrMatchNotFound):
		writeErr(w, http.StatusNotFound, reqID, "match_not_found", "Match not found.")
	case errors.Is(err, game.ErrMatchNotOwned):
		writeErr(w, http.StatusForbidden, reqID, "match_not_owned", "You are not the owner of this match.")
	case errors.Is(err, game.ErrMatchAlreadyEnded):
		writeErr(w, http.StatusConflict, reqID, "match_already_ended", "This match has already ended.")
	case errors.Is(err, game.ErrUnknownMap):
		writeErr(w, http.StatusBadRequest, reqID, "unknown_map", "Unknown map ID.")
	// Matchmaking errors
	case errors.Is(err, game.ErrAlreadyQueued):
		writeErr(w, http.StatusConflict, reqID, "already_queued", "You are already in the matchmaking queue.")
	// Chat errors
	case errors.Is(err, chat.ErrChannelNotFound):
		writeErr(w, http.StatusNotFound, reqID, "channel_not_found", "Chat channel not found.")
	case errors.Is(err, chat.ErrNotMember):
		writeErr(w, http.StatusForbidden, reqID, "not_member", "You are not a member of this channel.")
	case errors.Is(err, chat.ErrBodyEmpty):
		writeErr(w, http.StatusUnprocessableEntity, reqID, "body_empty", "Message body must not be empty.")
	case errors.Is(err, chat.ErrBodyTooLong):
		writeErr(w, http.StatusUnprocessableEntity, reqID, "body_too_long", "Message body exceeds 500 characters.")
	// Alliance errors
	case errors.Is(err, alliance.ErrNotFound):
		writeErr(w, http.StatusNotFound, reqID, "alliance_not_found", "Alliance not found.")
	case errors.Is(err, alliance.ErrNameTaken):
		writeErr(w, http.StatusConflict, reqID, "alliance_name_taken", "This alliance name is already taken.")
	case errors.Is(err, alliance.ErrTagTaken):
		writeErr(w, http.StatusConflict, reqID, "alliance_tag_taken", "This alliance tag is already taken.")
	case errors.Is(err, alliance.ErrAlreadyInAlliance):
		writeErr(w, http.StatusConflict, reqID, "already_in_alliance", "You are already a member of an alliance.")
	case errors.Is(err, alliance.ErrNotInAlliance):
		writeErr(w, http.StatusNotFound, reqID, "not_in_alliance", "You are not a member of any alliance.")
	case errors.Is(err, alliance.ErrNotMember):
		writeErr(w, http.StatusForbidden, reqID, "not_alliance_member", "You are not a member of this alliance.")
	case errors.Is(err, alliance.ErrPermissionDenied):
		writeErr(w, http.StatusForbidden, reqID, "alliance_permission_denied", "You do not have permission to perform this action.")
	case errors.Is(err, alliance.ErrLeaderMustTransfer):
		writeErr(w, http.StatusConflict, reqID, "leader_must_transfer", "Transfer leadership or disband the alliance before leaving.")
	case errors.Is(err, alliance.ErrInviteNotFound):
		writeErr(w, http.StatusNotFound, reqID, "invite_not_found", "Invite not found.")
	case errors.Is(err, alliance.ErrInviteNotPending):
		writeErr(w, http.StatusConflict, reqID, "invite_not_pending", "This invite is no longer pending.")
	case errors.Is(err, alliance.ErrAlreadyInvited):
		writeErr(w, http.StatusConflict, reqID, "already_invited", "This user already has a pending invite to your alliance.")
	case errors.Is(err, alliance.ErrCannotTargetSelf):
		writeErr(w, http.StatusUnprocessableEntity, reqID, "cannot_target_self", "You cannot target yourself with this action.")
	case errors.Is(err, alliance.ErrCannotKickLeader):
		writeErr(w, http.StatusForbidden, reqID, "cannot_kick_leader", "The alliance leader cannot be kicked.")
	// Event errors
	case errors.Is(err, events.ErrEventNotFound):
		writeErr(w, http.StatusNotFound, reqID, "event_not_found", "Event not found.")
	case errors.Is(err, events.ErrEventNotActive):
		writeErr(w, http.StatusConflict, reqID, "event_not_active", "This event is not currently active.")
	case errors.Is(err, events.ErrTierInvalid):
		writeErr(w, http.StatusBadRequest, reqID, "tier_invalid", "Tier index is out of range.")
	case errors.Is(err, events.ErrTierNotReached):
		writeErr(w, http.StatusConflict, reqID, "tier_not_reached", "You have not reached this reward tier yet.")
	case errors.Is(err, events.ErrTierAlreadyClaimed):
		writeErr(w, http.StatusConflict, reqID, "tier_already_claimed", "You have already claimed this reward tier.")
	default:
		slog.ErrorContext(r.Context(), "unhandled error", "err", err, "request_id", reqID)
		writeErr(w, http.StatusInternalServerError, reqID, "internal", "Something went wrong.")
	}
}

// writeErr is the low-level helper used by Error.
func writeErr(w http.ResponseWriter, status int, reqID, code, message string) {
	JSON(w, status, ErrorEnvelope{
		Error:     ErrorDetail{Code: code, Message: message},
		RequestID: reqID,
	})
}
