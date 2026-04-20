package middleware

import (
	"log/slog"
	"net/http"
)

// Recovery returns middleware that catches panics from downstream handlers,
// logs them as errors, and returns a plain 500 response so the server process
// does not crash.
func Recovery(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						"panic", rec,
						"request_id", RequestIDFromContext(r.Context()),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
