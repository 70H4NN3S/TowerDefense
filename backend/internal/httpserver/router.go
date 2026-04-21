package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/alliance"
	"github.com/70H4NN3S/TowerDefense/internal/auth"
	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/events"
	"github.com/70H4NN3S/TowerDefense/internal/game"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/handlers"
	"github.com/70H4NN3S/TowerDefense/internal/leaderboard"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
	"github.com/70H4NN3S/TowerDefense/internal/ws"
)

// resourceAwarder adapts game.ResourceService to the events.Awarder interface.
type resourceAwarder struct{ svc *game.ResourceService }

func (a resourceAwarder) AddGold(ctx context.Context, id uuid.UUID, n int64) error {
	_, err := a.svc.AddGold(ctx, id, n)
	return err
}

func (a resourceAwarder) AddDiamonds(ctx context.Context, id uuid.UUID, n int64) error {
	_, err := a.svc.AddDiamonds(ctx, id, n)
	return err
}

// registerRoutes wires all application routes onto mux.
// ctx governs the lifetime of the WebSocket hub.
// Handlers are thin: they decode, validate, call a service, and respond.
func registerRoutes(ctx context.Context, mux *http.ServeMux, pool *pgxpool.Pool, jwtSecret []byte) {
	mux.HandleFunc("GET /healthz", handleHealthz)

	authSvc := auth.NewService(pool, jwtSecret)
	authLimiter := middleware.NewIPLimiter(10, time.Minute)
	handlers.NewAuthHandler(authSvc, authLimiter).Register(mux)

	profileSvc := game.NewResourceService(pool)
	handlers.NewProfileHandler(profileSvc, jwtSecret).Register(mux)

	towerSvc := game.NewTowerService(pool, profileSvc)
	handlers.NewShopHandler(towerSvc, jwtSecret).Register(mux)
	handlers.NewTowerHandler(towerSvc, jwtSecret).Register(mux)

	matchSvc := game.NewMatchService(pool, profileSvc)
	handlers.NewMatchHandler(matchSvc, jwtSecret).Register(mux)

	matchStore := game.NewMatchStore(pool)
	hub := ws.NewHub()
	go hub.Run(ctx)

	sessionMgr := game.NewSessionManager(matchStore, profileSvc, hub, time.Now)
	matchmaker := game.NewMatchmaker(matchStore, sessionMgr, hub, time.Now)
	go matchmaker.Run(ctx)

	chatSvc := chat.NewService(pool, hub)

	// Multiplex incoming WS messages by type prefix to the appropriate service.
	sessionDispatch := sessionMgr.Dispatch
	chatDispatch := chatSvc.Dispatch
	hub.SetDispatch(func(userID uuid.UUID, msgType string, payload json.RawMessage) {
		switch {
		case strings.HasPrefix(msgType, "match."):
			sessionDispatch(userID, msgType, payload)
		case strings.HasPrefix(msgType, "chat."):
			chatDispatch(userID, msgType, payload)
		default:
			slog.Warn("ws: unhandled message type", "type", msgType, "user_id", userID)
		}
	})

	handlers.NewMatchmakingHandler(matchmaker, profileSvc, jwtSecret).Register(mux)
	handlers.NewChatHandler(chatSvc, jwtSecret).Register(mux)

	allianceSvc := alliance.NewService(pool, chatSvc)
	handlers.NewAllianceHandler(allianceSvc, jwtSecret).Register(mux)

	lbSvc := leaderboard.NewService(pool)
	lbSvc.StartRefresher(ctx, 5*time.Minute)
	handlers.NewLeaderboardHandler(lbSvc, jwtSecret).Register(mux)

	eventEngine := events.NewEngine(pool, resourceAwarder{svc: profileSvc})
	matchSvc.SetEventRecorder(eventEngine)
	handlers.NewEventHandler(eventEngine, jwtSecret).Register(mux)

	ws.NewHandler(hub, jwtSecret).Register(mux)
}
