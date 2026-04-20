package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// writeAuthErr writes a minimal JSON error envelope with the correct
// Content-Type. It is intentionally simple — the middleware package cannot
// import respond (circular dependency) so we write the fixed strings directly.
func writeAuthErr(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Safe to ignore: the header has already been sent.
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + message + `"}}`))
}

type userIDKey struct{}

// Authenticate returns a middleware that validates the Bearer token in the
// Authorization header. On success it stores the authenticated user ID in
// the request context; on failure it returns 401.
func Authenticate(jwtSecret []byte) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeAuthErr(w, http.StatusUnauthorized, "missing_token", "Authorization header required.")
				return
			}

			claims, err := auth.ParseToken(token, jwtSecret)
			if err != nil {
				writeAuthErr(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired.")
				return
			}

			userID, err := uuid.Parse(claims.Sub)
			if err != nil {
				writeAuthErr(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired.")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from the context.
// Returns (id, true) when the Authenticate middleware has set the value,
// and (zero, false) when it has not (unauthenticated route or missing middleware).
// Always check the bool to avoid silently acting on a zero UUID.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") {
		return "", false
	}
	tok := strings.TrimPrefix(v, "Bearer ")
	if tok == "" {
		return "", false
	}
	return tok, true
}
