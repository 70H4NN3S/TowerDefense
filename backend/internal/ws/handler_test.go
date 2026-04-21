package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/70H4NN3S/TowerDefense/internal/auth"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// testSecret is used to sign tokens in handler tests.
var testSecret = []byte("test-secret-for-ws-handler-tests")

// newTestToken creates a valid JWT for the given userID.
func newTestToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.SignToken(userID.String(), testSecret, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

// newTestServer spins up an httptest.Server with the WS handler and a real Hub.
func newTestServer(t *testing.T) (*httptest.Server, *Hub) {
	t.Helper()
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go hub.Run(ctx)

	mux := http.NewServeMux()
	NewHandler(hub, testSecret).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, hub
}

// wsURL converts an http:// test server URL to ws://.
func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestHandler_MissingToken_Returns401(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/v1/ws") //nolint:noctx
	if err != nil {
		t.Fatalf("GET /v1/ws: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandler_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/v1/ws?token=not-a-valid-jwt") //nolint:noctx
	if err != nil {
		t.Fatalf("GET /v1/ws: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandler_TokenInQueryParam_ConnectsSuccessfully(t *testing.T) {
	t.Parallel()

	srv, hub := newTestServer(t)
	userID := uuid.New()
	tok := newTestToken(t, userID)

	url := wsURL(srv, "/v1/ws") + "?token=" + tok
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v (status %v)", err, resp)
	}
	defer conn.Close()

	// Give the hub time to register the client.
	hub.Sync()

	// Server should accept (101 Switching Protocols — but Dial already verified).
	_ = resp
}

func TestHandler_TokenInProtocolHeader_ConnectsSuccessfully(t *testing.T) {
	t.Parallel()

	srv, hub := newTestServer(t)
	userID := uuid.New()
	tok := newTestToken(t, userID)

	header := http.Header{"Sec-Websocket-Protocol": []string{tok}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/v1/ws"), header)
	if err != nil {
		t.Fatalf("dial with protocol header: %v", err)
	}
	defer conn.Close()

	hub.Sync()
}

func TestHandler_PingPong_RoundTrip(t *testing.T) {
	t.Parallel()

	srv, hub := newTestServer(t)
	userID := uuid.New()
	tok := newTestToken(t, userID)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/v1/ws")+"?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	hub.Sync()

	// Send an application-level ping.
	ping, err := Marshal(TypePing, PingPayload{})
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, ping); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	// Expect an application-level pong back.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var payload PongPayload
	env, err := Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	if env.Type != TypePong {
		t.Errorf("type = %q, want %q", env.Type, TypePong)
	}
}

func TestHandler_HubSend_DeliveredToClient(t *testing.T) {
	t.Parallel()

	srv, hub := newTestServer(t)
	userID := uuid.New()
	tok := newTestToken(t, userID)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/v1/ws")+"?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	hub.Sync()

	// Push a message from the hub side.
	msg, err := Marshal(TypeError, ErrorPayload{Code: "test", Message: "hello"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	hub.Send(userID, msg)

	// The client should receive it.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read from hub send: %v", err)
	}

	var payload ErrorPayload
	env, err := Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("unmarshal hub message: %v", err)
	}
	if env.Type != TypeError || payload.Code != "test" {
		t.Errorf("unexpected message: type=%q code=%q", env.Type, payload.Code)
	}
}

func TestHandler_Disconnect_UnregistersClient(t *testing.T) {
	t.Parallel()

	srv, hub := newTestServer(t)
	userID := uuid.New()
	tok := newTestToken(t, userID)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, "/v1/ws")+"?token="+tok, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	hub.Sync()

	// Close from the client side.
	conn.Close()

	// Give the server-side read pump time to notice the disconnect.
	time.Sleep(50 * time.Millisecond)
	hub.Sync()

	// Sending to the user after disconnect should be a no-op (not panic).
	msg, _ := Marshal(TypePong, PongPayload{})
	hub.Send(userID, msg)
	hub.Sync()
}
