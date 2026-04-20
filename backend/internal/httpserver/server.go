// Package httpserver wires together the HTTP multiplexer, middleware stack, and
// route handlers into a ready-to-serve *http.Server.
package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/johannesniedens/towerdefense/internal/config"
	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

// New constructs and returns a configured *http.Server.
// pool may be nil; handlers that require the database will panic at route
// registration time if pool is nil when they are added.
func New(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) *http.Server {
	_ = pool // pool is unused in Phase 1; wired in as handlers are added.

	mux := http.NewServeMux()
	registerRoutes(mux)

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
