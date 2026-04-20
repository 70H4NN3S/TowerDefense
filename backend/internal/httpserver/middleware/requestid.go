package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
)

type reqIDKeyType struct{}

var reqIDKey reqIDKeyType

// RequestID reads the incoming X-Request-ID header (or generates a new 16-byte
// hex ID if absent), stores it on the context, and echoes it in the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), reqIDKey, id),
		))
	})
}

// RequestIDFromContext retrieves the request ID stored by RequestID middleware.
// It returns an empty string when the middleware was not applied.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(reqIDKey).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// crypto/rand failure is catastrophic; panic is appropriate here.
		panic("requestid: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
