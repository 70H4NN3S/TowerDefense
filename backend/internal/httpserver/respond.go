package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

// errorEnvelope is the standard API error response shape described in
// .claude/rules/error-handling.md.
type errorEnvelope struct {
	Error     errorDetail `json:"error"`
	RequestID string      `json:"request_id,omitempty"`
}

// errorDetail carries the machine-readable code and a user-safe message.
type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// RespondJSON sets Content-Type to application/json, writes status, and JSON-
// encodes v into the response body. Encoding errors are logged but not surfaced
// to the caller because the header has already been sent.
func RespondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode response", "err", err)
	}
}

// RespondError maps err to an HTTP status and writes a JSON error envelope.
// Domain-specific mappings (ErrNotFound, ErrInsufficientFunds, …) will be added
// as sentinel errors are introduced in later phases.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.RequestIDFromContext(r.Context())
	slog.ErrorContext(r.Context(), "unhandled error", "err", err, "request_id", reqID)
	respondErr(w, http.StatusInternalServerError, reqID, "internal", "Something went wrong.")
}

// respondErr is the low-level helper used by RespondError and, in future phases,
// the domain-specific branches.
func respondErr(w http.ResponseWriter, status int, reqID, code, message string) {
	RespondJSON(w, status, errorEnvelope{
		Error:     errorDetail{Code: code, Message: message},
		RequestID: reqID,
	})
}
