package ws

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is how often we send a ping. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageBytes is the maximum size of a message from a client.
	maxMessageBytes = 4096

	// sendBufferSize is the number of messages that can be queued for a client
	// before backpressure kicks in.
	sendBufferSize = 256
)

// Client represents one WebSocket connection for an authenticated user.
type Client struct {
	userID uuid.UUID
	hub    *Hub
	conn   *websocket.Conn

	// send is a buffered channel of outbound messages. Hub.route writes here;
	// writePump drains it to the wire.
	send chan []byte
}

// newClient creates a Client and its send channel.
func newClient(userID uuid.UUID, hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		userID: userID,
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
	}
}

// ReadPump pumps messages from the WebSocket connection into the hub.
// It runs in a per-connection goroutine. When the pump exits it unregisters
// the client so writePump will also exit (its send channel gets closed).
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("ws: unexpected close", "user_id", c.userID, "err", err)
			}
			break
		}

		// Parse the envelope and handle application-level ping.
		env, err := Unmarshal(data, nil)
		if err != nil {
			slog.Warn("ws: malformed message", "user_id", c.userID, "err", err)
			continue
		}

		switch env.Type {
		case TypePing:
			pong, merr := Marshal(TypePong, PongPayload{})
			if merr != nil {
				slog.Error("ws: marshal pong", "err", merr)
				continue
			}
			select {
			case c.send <- pong:
			default:
				slog.Warn("ws: send buffer full dropping pong", "user_id", c.userID)
			}
		default:
			// Route to the registered dispatcher (e.g. game session manager).
			c.hub.Dispatch(c.userID, env.Type, env.Payload)
		}
	}
}

// WritePump pumps messages from the send channel to the WebSocket connection.
// It runs in a per-connection goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel — send a close frame and exit.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("ws: write error", "user_id", c.userID, "err", err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Warn("ws: ping error", "user_id", c.userID, "err", err)
				return
			}
		}
	}
}
