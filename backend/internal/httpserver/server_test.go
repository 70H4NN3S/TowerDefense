package httpserver_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/config"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{ListenAddr: ":0", LogLevel: "error"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httpserver.New(t.Context(), cfg, log, nil)
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
	if body["version"] == "" {
		t.Error("version field must not be empty")
	}
}

func TestHealthz_SetsRequestIDHeader(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID response header missing")
	}
}

func TestHealthz_SetsCORSHeader(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Access-Control-Allow-Origin header missing or wrong")
	}
}
