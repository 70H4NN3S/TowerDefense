package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

type userIDKey struct{}

// Authenticate returns a middleware that validates the Bearer token in the
// Authorization header. On success it stores the authenticated user ID in
// the request context; on failure it returns 401.
func Authenticate(jwtSecret []byte) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				http.Error(w, `{"error":{"code":"missing_token","message":"Authorization header required."}}`, http.StatusUnauthorized)
				return
			}

			claims, err := auth.ParseToken(token, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":{"code":"invalid_token","message":"Token is invalid or expired."}}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.Sub)
			if err != nil {
				http.Error(w, `{"error":{"code":"invalid_token","message":"Token is invalid or expired."}}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from the request
// context. Returns the zero UUID if the context carries no user ID (i.e.
// the request did not pass through Authenticate middleware).
func UserIDFromContext(r *http.Request) uuid.UUID {
	id, _ := r.Context().Value(userIDKey{}).(uuid.UUID)
	return id
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
