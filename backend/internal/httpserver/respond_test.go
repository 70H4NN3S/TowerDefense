package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON(t *testing.T) {
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
			RespondJSON(w, tt.status, tt.body)

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

func TestRespondError_WritesInternalEnvelope(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	RespondError(w, r, errors.New("something broke"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var env errorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error.Code != "internal" {
		t.Errorf("error.code = %q, want %q", env.Error.Code, "internal")
	}
	if env.Error.Message == "" {
		t.Error("error.message must not be empty")
	}
}

func TestRespondError_NeverLeaksInternalDetails(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	sensitiveErr := errors.New("pq: duplicate key value violates unique constraint users_email_key")
	RespondError(w, r, sensitiveErr)

	body := w.Body.String()
	if contains(body, "pq:") || contains(body, "unique constraint") {
		t.Errorf("response body contains internal error details: %s", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRune(s, sub))
}

func containsRune(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
