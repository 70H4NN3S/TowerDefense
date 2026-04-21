package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/config"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/handlers"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/testutil/authtest"
)

// newAuthTestServer builds a full httptest.Server with the auth handler wired in.
// It uses a nil pgxpool.Pool — the auth service is backed by fakeStore — but since
// we construct Service/AuthHandler directly, the pool is not used.
func newAuthTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := &config.Config{
		ListenAddr: ":0",
		LogLevel:   "error",
		JWTSecret:  "test-secret-32-bytes-for-testing!",
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Build a service with an in-memory store (no real DB).
	svc := authtest.NewService([]byte(cfg.JWTSecret))

	mux := http.NewServeMux()
	limiter := middleware.NewIPLimiter(100, time.Minute) // high limit so tests don't hit 429
	handlers.NewAuthHandler(svc, limiter).Register(mux)

	handler := middleware.Chain(
		mux,
		middleware.RequestID,
		middleware.Logger(log),
		middleware.Recovery(log),
		middleware.CORS,
		middleware.RequireJSON,
	)
	srv := httpserver.NewWithHandler(cfg, handler)
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func jsonBody(v any) *bytes.Buffer {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(b)
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body any) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- /v1/auth/register ---

func TestRegister_Returns201WithTokens(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp := postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "alice@example.com",
		"username": "alice",
		"password": "correct-horse-battery",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["access_token"] == "" {
		t.Error("access_token must not be empty")
	}
	if body["refresh_token"] == "" {
		t.Error("refresh_token must not be empty")
	}
}

func TestRegister_ValidationError_Returns400(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp := postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "notanemail",
		"username": "",
		"password": "short",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	var body map[string]any
	decodeJSON(t, resp, &body)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatal("response missing 'error' object")
	}
	if errObj["code"] != "validation_failed" {
		t.Errorf("error.code = %v, want validation_failed", errObj["code"])
	}
}

func TestRegister_DuplicateEmail_Returns409(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	body := map[string]string{
		"email":    "dup@example.com",
		"username": "dup",
		"password": "correct-horse-battery",
	}
	postJSON(t, ts, "/v1/auth/register", body).Body.Close()

	resp := postJSON(t, ts, "/v1/auth/register", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

// --- /v1/auth/login ---

func TestLogin_HappyPath(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "bob@example.com",
		"username": "bob",
		"password": "correct-horse-battery",
	}).Body.Close()

	resp := postJSON(t, ts, "/v1/auth/login", map[string]string{
		"email":    "bob@example.com",
		"password": "correct-horse-battery",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "carol@example.com",
		"username": "carol",
		"password": "correct-horse-battery",
	}).Body.Close()

	resp := postJSON(t, ts, "/v1/auth/login", map[string]string{
		"email":    "carol@example.com",
		"password": "wrong-password!!!!",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLogin_UnknownEmail_Returns401(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp := postJSON(t, ts, "/v1/auth/login", map[string]string{
		"email":    "nobody@example.com",
		"password": "correct-horse-battery",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no user enumeration)", resp.StatusCode)
	}
}

// --- /v1/auth/refresh ---

func TestRefresh_HappyPath(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	var reg map[string]any
	r := postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "dave@example.com",
		"username": "dave",
		"password": "correct-horse-battery",
	})
	decodeJSON(t, r, &reg)

	refreshToken, _ := reg["refresh_token"].(string)
	resp := postJSON(t, ts, "/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRefresh_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp := postJSON(t, ts, "/v1/auth/refresh", map[string]string{
		"refresh_token": "not.a.token",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// --- malformed body ---

func TestRegister_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp, err := http.Post(ts.URL+"/v1/auth/register", "application/json",
		strings.NewReader("{not valid json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestLogin_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp, err := http.Post(ts.URL+"/v1/auth/login", "application/json",
		strings.NewReader("{not valid json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRefresh_MalformedJSON_Returns400(t *testing.T) {
	t.Parallel()

	ts := newAuthTestServer(t)
	resp, err := http.Post(ts.URL+"/v1/auth/refresh", "application/json",
		strings.NewReader("{not valid json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// --- rate limit ---

// newTightLimiterServer returns a test server whose auth limiter allows only
// capacity requests per minute. Callers use this to trigger 429 responses.
func newTightLimiterServer(t *testing.T, capacity int) *httptest.Server {
	t.Helper()

	cfg := &config.Config{ListenAddr: ":0", LogLevel: "error", JWTSecret: "test-secret-32-bytes-for-testing!"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := authtest.NewService([]byte(cfg.JWTSecret))

	mux := http.NewServeMux()
	limiter := middleware.NewIPLimiter(capacity, time.Minute)
	handlers.NewAuthHandler(svc, limiter).Register(mux)

	handler := middleware.Chain(mux, middleware.RequestID, middleware.Logger(log), middleware.Recovery(log), middleware.CORS, middleware.RequireJSON)
	ts := httptest.NewServer(httpserver.NewWithHandler(cfg, handler).Handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestRegister_RateLimit_Returns429(t *testing.T) {
	t.Parallel()

	ts := newTightLimiterServer(t, 2)

	body := func(i int) *bytes.Buffer {
		return jsonBody(map[string]string{
			"email":    strings.Repeat("a", i) + "@example.com",
			"username": strings.Repeat("u", i+1),
			"password": "correct-horse-battery",
		})
	}

	for i := range 2 {
		resp, _ := http.Post(ts.URL+"/v1/auth/register", "application/json", body(i))
		resp.Body.Close()
	}

	resp, err := http.Post(ts.URL+"/v1/auth/register", "application/json", body(99))
	if err != nil {
		t.Fatalf("POST /v1/auth/register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
}

func TestRefresh_RateLimit_Returns429(t *testing.T) {
	t.Parallel()

	ts := newTightLimiterServer(t, 2)

	// Register once (consumes 1 of 2 allowed requests).
	var reg map[string]any
	r := postJSON(t, ts, "/v1/auth/register", map[string]string{
		"email":    "refresh-limit@example.com",
		"username": "refreshlimituser",
		"password": "correct-horse-battery",
	})
	decodeJSON(t, r, &reg)

	refreshToken, _ := reg["refresh_token"].(string)

	// Second request — exhausts the bucket.
	postJSON(t, ts, "/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	}).Body.Close()

	// Third request must be rate-limited.
	resp := postJSON(t, ts, "/v1/auth/refresh", map[string]string{
		"refresh_token": refreshToken,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 (refresh must be rate-limited)", resp.StatusCode)
	}
}
