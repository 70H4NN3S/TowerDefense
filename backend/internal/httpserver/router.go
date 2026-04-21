package httpserver

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/game"
	"github.com/johannesniedens/towerdefense/internal/httpserver/handlers"
	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
)

// registerRoutes wires all application routes onto mux.
// Handlers are thin: they decode, validate, call a service, and respond.
func registerRoutes(mux *http.ServeMux, pool *pgxpool.Pool, jwtSecret []byte) {
	mux.HandleFunc("GET /healthz", handleHealthz)

	authSvc := auth.NewService(pool, jwtSecret)
	authLimiter := middleware.NewIPLimiter(10, time.Minute)
	handlers.NewAuthHandler(authSvc, authLimiter).Register(mux)

	profileSvc := game.NewResourceService(pool)
	handlers.NewProfileHandler(profileSvc, jwtSecret).Register(mux)

	towerSvc := game.NewTowerService(pool, profileSvc)
	handlers.NewShopHandler(towerSvc, jwtSecret).Register(mux)
	handlers.NewTowerHandler(towerSvc, jwtSecret).Register(mux)
}
