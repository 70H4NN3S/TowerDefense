package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

func TestAllow_UnderLimit(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(5, time.Minute)
	for i := range 5 {
		allowed, retry := l.Allow("1.2.3.4")
		if !allowed {
			t.Errorf("request %d: want allowed, got denied (retry=%d)", i+1, retry)
		}
	}
}

func TestAllow_BlocksOverLimit(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(3, time.Minute)
	for range 3 {
		l.Allow("10.0.0.1")
	}
	allowed, retry := l.Allow("10.0.0.1")
	if allowed {
		t.Error("4th request: want denied, got allowed")
	}
	if retry < 1 {
		t.Errorf("retry = %d, want >= 1", retry)
	}
}

func TestAllow_RetryAfterIsAccurate(t *testing.T) {
	t.Parallel()

	// 1 req/min → rate = 1/60 tokens/sec → 1 token takes 60 s to refill.
	l := middleware.NewIPLimiter(1, time.Minute)
	l.Allow("5.5.5.5") // exhaust the single token

	allowed, retry := l.Allow("5.5.5.5")
	if allowed {
		t.Fatal("want denied, got allowed")
	}
	// ceil((1 - 0) / (1/60)) = 60 seconds
	if retry != 60 {
		t.Errorf("retry = %d, want 60", retry)
	}
}

func TestAllow_IndependentPerIP(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(2, time.Minute)
	l.Allow("192.168.1.1")
	l.Allow("192.168.1.1")

	allowed, _ := l.Allow("192.168.1.2")
	if !allowed {
		t.Error("different IP should be allowed")
	}
}

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

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = addr
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}

	raw := w.Header().Get("Retry-After")
	if raw == "" {
		t.Fatal("Retry-After header missing on 429 response")
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		t.Errorf("Retry-After = %q: not an integer: %v", raw, err)
	} else if v < 1 {
		t.Errorf("Retry-After = %d, want >= 1", v)
	}
}

func TestRateLimit_IndependentPerIP(t *testing.T) {
	t.Parallel()

	l := middleware.NewIPLimiter(2, time.Minute)
	handler := middleware.RateLimit(l)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range 2 {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "192.168.1.1:1000"
		handler.ServeHTTP(w, r)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.2:1000"
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("different IP status = %d, want 200", w.Code)
	}
}
