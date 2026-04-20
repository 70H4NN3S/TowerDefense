package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

var authTestSecret = []byte("auth-test-secret-32-bytes-here!!")

const authTestSubject = "550e8400-e29b-41d4-a716-446655440000"

func validToken(t *testing.T) string {
	t.Helper()
	tok, err := auth.SignToken(authTestSubject, authTestSecret, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

func authHandler(secret []byte) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := middleware.UserIDFromContext(r.Context())
		w.Header().Set("X-User-ID", id.String())
		w.WriteHeader(http.StatusOK)
	})
	return middleware.Authenticate(secret)(inner)
}

func TestAuthenticate_ValidToken(t *testing.T) {
	t.Parallel()

	tok := validToken(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	authHandler(authTestSecret).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	gotID := w.Header().Get("X-User-ID")
	if gotID != authTestSubject {
		t.Errorf("user ID = %q, want %q", gotID, authTestSubject)
	}
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	authHandler(authTestSecret).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthenticate_WrongSecret(t *testing.T) {
	t.Parallel()

	tok := validToken(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	authHandler([]byte("wrong-secret-key-32-bytes-here!!")).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthenticate_ExpiredToken(t *testing.T) {
	t.Parallel()

	// Sign with negative TTL so it's already expired.
	tok, err := auth.SignToken(authTestSubject, authTestSecret, -time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	authHandler(authTestSecret).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthenticate_BearerPrefixRequired(t *testing.T) {
	t.Parallel()

	tok := validToken(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", tok) // no "Bearer " prefix

	authHandler(authTestSecret).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestUserIDFromContext_ReturnsZeroAndFalseWhenAbsent(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	id, ok := middleware.UserIDFromContext(r.Context())
	if ok {
		t.Error("UserIDFromContext on empty context: ok = true, want false")
	}
	if id != (uuid.UUID)("") {
		t.Errorf("UserIDFromContext on empty context: id = %q, want zero value", id)
	}
}
