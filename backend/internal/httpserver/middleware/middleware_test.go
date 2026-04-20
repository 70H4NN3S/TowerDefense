package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

// echoHandler returns a handler that writes the given status code.
func echoHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

// --- Logger ---

func TestLogger_CapturesStatusCode(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, nil))

	handler := middleware.Logger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", w.Code)
	}
	logged := buf.String()
	if !strings.Contains(logged, "418") {
		t.Errorf("log output does not contain status 418: %s", logged)
	}
	if !strings.Contains(logged, "/test") {
		t.Errorf("log output does not contain path /test: %s", logged)
	}
}

func TestLogger_DefaultStatus200WhenWriteHeaderNotCalled(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	log := slog.New(slog.NewTextHandler(&buf, nil))

	handler := middleware.Logger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write without calling WriteHeader — status should default to 200.
		_, _ = w.Write([]byte("ok"))
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if !strings.Contains(buf.String(), "200") {
		t.Errorf("log output does not contain default status 200: %s", buf.String())
	}
}

// --- Chain ---

func TestChain_ExecutesInOrder(t *testing.T) {
	t.Parallel()

	order := make([]int, 0, 3)
	mkMiddleware := func(n int) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, n)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := middleware.Chain(echoHandler(http.StatusOK),
		mkMiddleware(1), mkMiddleware(2), mkMiddleware(3))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("execution order = %v, want [1 2 3]", order)
	}
}

// --- RequestID ---

func TestRequestID_GeneratesIDWhenAbsent(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	middleware.RequestID(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestRequestID_PropagatesProvidedID(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "my-custom-id")

	var gotID string
	handler := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotID = middleware.RequestIDFromContext(req.Context())
	}))
	handler.ServeHTTP(w, r)

	if gotID != "my-custom-id" {
		t.Errorf("context ID = %q, want %q", gotID, "my-custom-id")
	}
	if w.Header().Get("X-Request-ID") != "my-custom-id" {
		t.Errorf("response header = %q, want %q", w.Header().Get("X-Request-ID"), "my-custom-id")
	}
}

func TestRequestIDFromContext_EmptyWhenNotSet(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := middleware.RequestIDFromContext(r.Context()); id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// --- Recovery ---

func TestRecovery_CatchesPanic(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware.Recovery(log)(panicHandler).ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- CORS ---

func TestCORS_SetsHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware.CORS(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Access-Control-Allow-Origin missing or wrong")
	}
}

func TestCORS_OptionsReturns204(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	middleware.CORS(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

// --- RequireJSON ---

func TestRequireJSON_AllowsGET(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware.RequireJSON(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("GET status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireJSON_AllowsPostWithJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Content-Type", "application/json")
	middleware.RequireJSON(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("POST with JSON status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireJSON_RejectsPostWithoutJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	middleware.RequireJSON(echoHandler(http.StatusOK)).ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("POST without Content-Type status = %d, want %d",
			w.Code, http.StatusUnsupportedMediaType)
	}
}
