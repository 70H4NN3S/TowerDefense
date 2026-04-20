package middleware

import (
	"net/http"
	"strings"
)

// RequireJSON rejects write requests (POST, PUT, PATCH) whose Content-Type is
// not application/json with a 415 Unsupported Media Type. GET, HEAD, and OPTIONS
// are always allowed through.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		next.ServeHTTP(w, r)
	})
}
