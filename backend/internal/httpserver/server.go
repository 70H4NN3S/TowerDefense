// Package httpserver wires together the HTTP multiplexer, middleware stack, and
// route handlers into a ready-to-serve *http.Server.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/config"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
)

// New constructs and returns a configured *http.Server.
// ctx governs the lifetime of background goroutines (e.g. the WebSocket hub).
// pool may be nil; handlers that require the database will panic at route
// registration time if pool is nil when they are added.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) *http.Server {
	mux := http.NewServeMux()
	registerRoutes(ctx, mux, pool, []byte(cfg.JWTSecret))

	handler := middleware.Chain(
		mux,
		middleware.RequestID,
		middleware.Logger(log),
		middleware.Recovery(log),
		middleware.CORS,
		middleware.RequireJSON,
	)

	return &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// NewWithHandler constructs an *http.Server using the provided handler
// instead of the default route set. Intended for tests that supply their
// own mux or middleware stack.
func NewWithHandler(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
