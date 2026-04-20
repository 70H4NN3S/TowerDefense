package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(5, time.Minute)
	handler := middleware.RateLimit(l)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := range 5 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "1.2.3.4:9000"
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(3, time.Minute)
	handler := middleware.RateLimit(l)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	addr := "10.0.0.1:1234"
	for range 3 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = addr
		handler.ServeHTTP(w, r)
	}

	// 4th request should be rejected.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = addr
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing on 429 response")
	}
}

func TestRateLimit_IndependentPerIP(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(2, time.Minute)
	handler := middleware.RateLimit(l)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP A.
	for range 2 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "192.168.1.1:1000"
		handler.ServeHTTP(w, r)
	}

	// IP B should still be allowed.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.2:1000"
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("different IP status = %d, want 200", w.Code)
	}
}
