package ws

import (
	"context"
	"testing"

	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// makeTestClient builds a minimal Client suitable for hub tests (no real conn).
func makeTestClient(hub *Hub) (*Client, uuid.UUID) {
	id := uuid.New()
	c := &Client{
		userID: id,
		hub:    hub,
		send:   make(chan []byte, sendBufferSize),
	}
	return c, id
}

func TestHub_RegisterUnregister(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub()
	go h.Run(ctx)

	c, uid := makeTestClient(h)
	h.Register(c)
	h.Sync()

	// Registered client should receive messages.
	msg := []byte("hello")
	h.Send(uid, msg)
	h.Sync()

	select {
	case got := <-c.send:
		if string(got) != string(msg) {
			t.Errorf("got %q, want %q", got, msg)
		}
	default:
		t.Error("no message after Register")
	}

	h.Unregister(c)
	h.Sync()

	// send channel must be closed after unregister.
	select {
	case _, open := <-c.send:
		if open {
			t.Error("send channel should be closed")
		}
	default:
		t.Error("send channel was not closed after Unregister")
	}
}

func TestHub_Send_UnknownUser(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub()
	go h.Run(ctx)

	// Sending to an unregistered user must not panic or block.
	h.Send(uuid.New(), []byte(`{}`))
	h.Sync() // confirm the hub processed it without error
}

func TestHub_MultipleClientsForUser(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub()
	go h.Run(ctx)

	uid := uuid.New()
	c1 := &Client{userID: uid, hub: h, send: make(chan []byte, sendBufferSize)}
	c2 := &Client{userID: uid, hub: h, send: make(chan []byte, sendBufferSize)}
	h.Register(c1)
	h.Register(c2)

	msg := []byte(`{"v":1,"type":"pong","payload":{}}`)
	h.Send(uid, msg)
	h.Sync()

	for i, c := range []*Client{c1, c2} {
		select {
		case got := <-c.send:
			if string(got) != string(msg) {
				t.Errorf("client %d: got %q, want %q", i, got, msg)
			}
		default:
			t.Errorf("client %d: did not receive message", i)
		}
	}
}

func TestHub_Shutdown_ClosesAllChannels(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	h := NewHub()
	go h.Run(ctx)

	c1, _ := makeTestClient(h)
	c2, _ := makeTestClient(h)
	h.Register(c1)
	h.Register(c2)
	h.Sync()

	cancel() // stop the hub

	// Both send channels should be closed. Use a helper that retries briefly
	// because Run runs in another goroutine.
	for i, c := range []*Client{c1, c2} {
		// Drain up to sendBufferSize messages (none expected) then expect close.
		closed := false
		for range sendBufferSize + 1 {
			v, open := <-c.send
			if !open {
				closed = true
				break
			}
			_ = v
		}
		if !closed {
			t.Errorf("client %d: send channel not closed after shutdown", i)
		}
	}
}

func TestHub_DoubleUnregister(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := NewHub()
	go h.Run(ctx)

	c, _ := makeTestClient(h)
	h.Register(c)
	h.Sync()

	h.Unregister(c)
	h.Sync()

	// Second unregister on an already-removed client must not panic.
	// We pass the same client pointer; the hub guards against double-removal.
	h.Unregister(c)
	h.Sync()
}
