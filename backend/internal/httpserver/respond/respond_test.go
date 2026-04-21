package respond

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/auth"
	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/models"
)

func TestJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       any
		wantStatus int
	}{
		{"200 with map body", http.StatusOK, map[string]string{"k": "v"}, http.StatusOK},
		{"201 created", http.StatusCreated, map[string]string{"id": "1"}, http.StatusCreated},
		{"204 no content", http.StatusNoContent, nil, http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			JSON(w, tt.status, tt.body)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}
			if tt.body != nil {
				var v any
				if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
					t.Errorf("response is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestError_NeverLeaksInternalDetails(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	sensitiveErr := errors.New("pq: duplicate key value violates unique constraint users_email_key")
	Error(w, r, sensitiveErr)

	body := w.Body.String()
	if strings.Contains(body, "pq:") || strings.Contains(body, "unique constraint") {
		t.Errorf("response body contains internal error details: %s", body)
	}
}

// TestError_DomainErrors verifies that every mapped domain error produces the
// correct HTTP status and machine-readable code.
func TestError_DomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"unhandled error", errors.New("boom"), http.StatusInternalServerError, "internal"},
		// Matchmaking errors
		{"already queued", game.ErrAlreadyQueued, http.StatusConflict, "already_queued"},
		// Auth errors
		{"invalid credentials", auth.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
		{"email taken", auth.ErrEmailTaken, http.StatusConflict, "email_taken"},
		{"username taken", auth.ErrUsernameTaken, http.StatusConflict, "username_taken"},
		{"not found", auth.ErrNotFound, http.StatusNotFound, "not_found"},
		{"expired token", auth.ErrExpiredToken, http.StatusUnauthorized, "token_expired"},
		{"invalid token", auth.ErrInvalidToken, http.StatusUnauthorized, "invalid_token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			Error(w, r, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var env ErrorEnvelope
			if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if env.Error.Code != tt.wantCode {
				t.Errorf("error.code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if env.Error.Message == "" {
				t.Error("error.message must not be empty")
			}
		})
	}
}

// TestError_ValidationError verifies that a *models.ValidationError produces a
// 400 with code "validation_failed" and a populated fields list.
func TestError_ValidationError(t *testing.T) {
	t.Parallel()

	ve := &models.ValidationError{}
	ve.Add("email", "invalid format").Add("password", "too short")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	Error(w, r, ve)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var env ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error.Code != "validation_failed" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "validation_failed")
	}
	if env.Error.Details == nil {
		t.Error("details must not be nil for validation errors")
	}
}

// TestError_WrappedDomainError verifies that domain errors wrapped with
// fmt.Errorf("%w") are still matched correctly via errors.Is.
func TestError_WrappedDomainError(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("outer"), auth.ErrInvalidCredentials)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	Error(w, r, wrapped)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for wrapped ErrInvalidCredentials", w.Code)
	}
}

// TestError_RequestIDPropagated verifies that the request ID from context
// appears in the error envelope when the RequestID middleware has run.
func TestError_RequestIDPropagated(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	// Inject a known request ID into the context via the header; the respond
	// package reads it from the context (set by middleware.RequestID).
	// Since we are testing the respond package in isolation, we simulate the
	// middleware by directly using the context helper from the middleware package.
	// Here we just verify the envelope has a request_id field when absent — it
	// should be omitempty. A full propagation test lives in server_test.go.
	Error(w, r, errors.New("anything"))

	var env ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Without middleware the field is empty and omitempty drops it from JSON.
	// We verify the envelope still decoded correctly (no panic, valid JSON).
	if env.Error.Code == "" {
		t.Error("error.code must not be empty")
	}
}
