package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// hubOp is the internal operation type processed by Hub.Run.
type hubOp int

const (
	opRegister   hubOp = iota
	opUnregister       // signals Hub.Run to unregister and close client.send
	opSend             // route a message to a specific user
	opSync             // used in tests to flush the ops queue
)

// hubMsg is a single operation passed through Hub.ops.
type hubMsg struct {
	op     hubOp
	client *Client
	// send fields
	userID uuid.UUID
	data   []byte
	// sync fields: closed by Run after processing all preceding ops
	syncDone chan struct{}
}

// Hub maintains the set of active WebSocket clients and routes messages.
// All mutations happen inside the single Run goroutine so no mutex is needed
// for the clients map.
type Hub struct {
	// ops is the single channel through which all Hub state changes flow.
	ops chan hubMsg

	// clients maps user ID → set of active connections. One user can hold
	// multiple connections (e.g. reconnect before the old TCP conn times out).
	clients map[uuid.UUID]map[*Client]bool

	// dispatch is called for every incoming message that the hub does not
	// handle internally. Set once before Run; never mutated afterward.
	dispatch DispatchFunc
}

// NewHub constructs a ready-to-use Hub. Call Run to start the event loop.
func NewHub() *Hub {
	return &Hub{
		ops:     make(chan hubMsg, 64),
		clients: make(map[uuid.UUID]map[*Client]bool),
	}
}

// SetDispatch wires the function that receives non-internal incoming messages.
// Must be called before Run starts.
func (h *Hub) SetDispatch(d DispatchFunc) {
	h.dispatch = d
}

// Dispatch calls the registered DispatchFunc for the given message.
// Safe to call from any goroutine.
func (h *Hub) Dispatch(userID uuid.UUID, msgType string, payload json.RawMessage) {
	if h.dispatch != nil {
		h.dispatch(userID, msgType, payload)
	}
}

// Run processes hub operations until ctx is cancelled. It must be called in
// its own goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Close every active client send channel so write pumps drain.
			for _, set := range h.clients {
				for c := range set {
					close(c.send)
				}
			}
			h.clients = make(map[uuid.UUID]map[*Client]bool)
			return

		case msg := <-h.ops:
			switch msg.op {
			case opRegister:
				h.register(msg.client)
			case opUnregister:
				h.unregister(msg.client)
			case opSend:
				h.route(msg.userID, msg.data)
			case opSync:
				close(msg.syncDone)
			}
		}
	}
}

// Register enqueues a client for registration. Non-blocking; returns
// immediately. The actual map update happens inside Run.
func (h *Hub) Register(c *Client) {
	h.ops <- hubMsg{op: opRegister, client: c}
}

// Unregister enqueues a client for removal.
func (h *Hub) Unregister(c *Client) {
	h.ops <- hubMsg{op: opUnregister, client: c}
}

// Send queues data to be delivered to all connections owned by userID.
// Returns without blocking; messages are dropped if the hub ops buffer is full.
func (h *Hub) Send(userID uuid.UUID, data []byte) {
	select {
	case h.ops <- hubMsg{op: opSend, userID: userID, data: data}:
	default:
		slog.Warn("ws: hub ops buffer full, dropping send", "user_id", userID)
	}
}

// register adds a client to the clients map.
func (h *Hub) register(c *Client) {
	if _, ok := h.clients[c.userID]; !ok {
		h.clients[c.userID] = make(map[*Client]bool)
	}
	h.clients[c.userID][c] = true
	slog.Info("ws: client registered", "user_id", c.userID, "total", len(h.clients[c.userID]))
}

// unregister removes a client and closes its send channel.
func (h *Hub) unregister(c *Client) {
	set, ok := h.clients[c.userID]
	if !ok {
		return
	}
	if _, exists := set[c]; !exists {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, c.userID)
	}
	close(c.send)
	slog.Info("ws: client unregistered", "user_id", c.userID)
}

// Sync blocks until the hub has processed all operations enqueued before this
// call. It is intended for use in tests only.
func (h *Hub) Sync() {
	done := make(chan struct{})
	h.ops <- hubMsg{op: opSync, syncDone: done}
	<-done
}

// route delivers data to every connection for userID, dropping individual
// client queues that are full (backpressure).
func (h *Hub) route(userID uuid.UUID, data []byte) {
	set, ok := h.clients[userID]
	if !ok {
		return
	}
	for c := range set {
		select {
		case c.send <- data:
		default:
			slog.Warn("ws: client send buffer full, dropping message", "user_id", userID)
		}
	}
}
