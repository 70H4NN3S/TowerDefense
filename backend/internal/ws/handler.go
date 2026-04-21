package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin allows connections from any origin because our primary
	// client is the Capacitor mobile app (not a browser page). A production
	// deployment behind an API gateway can enforce stricter origin checks.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler holds the dependencies needed to upgrade HTTP connections to WS.
type Handler struct {
	hub       *Hub
	jwtSecret []byte
}

// NewHandler constructs a Handler.
func NewHandler(hub *Hub, jwtSecret []byte) *Handler {
	return &Handler{hub: hub, jwtSecret: jwtSecret}
}

// Register wires the WS endpoint onto mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/ws", h.handleUpgrade)
}

// handleUpgrade authenticates the request, upgrades to WebSocket, and starts
// the client pumps.
//
// Authentication accepts a JWT token in two places (in order of precedence):
//  1. Query parameter: ?token=<jwt>
//  2. Sec-WebSocket-Protocol header value (set to the token string), e.g.
//     Sec-WebSocket-Protocol: <jwt>
//
// The query-parameter approach is simpler for mobile clients; the protocol
// header approach works in browsers where custom headers on WS connections
// are restricted.
func (h *Handler) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	token, ok := extractToken(r)
	if !ok {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseToken(token, h.jwtSecret)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.Sub)
	if err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade writes its own error response on failure; just log and return.
		return
	}

	client := newClient(userID, h.hub, conn)
	h.hub.Register(client)

	// writePump owns its goroutine; readPump runs in the caller goroutine.
	go client.WritePump()
	client.ReadPump()
}

// extractToken returns the JWT token string from the request, checking the
// query parameter first, then the Sec-WebSocket-Protocol header.
func extractToken(r *http.Request) (string, bool) {
	if tok := r.URL.Query().Get("token"); tok != "" {
		return tok, true
	}

	// Sec-WebSocket-Protocol carries a comma-separated list of sub-protocols.
	// Clients that can't set custom headers encode the token as the sole
	// sub-protocol value.
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		// Trim any whitespace around the value.
		tok := strings.TrimSpace(proto)
		if tok != "" {
			return tok, true
		}
	}

	return "", false
}
