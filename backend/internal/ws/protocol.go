// Package ws implements the WebSocket hub, client pumps, and message protocol.
package ws

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// Version is the current protocol version. It is included in every envelope
// so clients can reject messages from an incompatible server.
const Version = 1

// Known message type strings.
const (
	TypePing  = "ping"
	TypePong  = "pong"
	TypeError = "error"

	// Multiplayer match messages.
	TypeMatchFound    = "match.found"
	TypeMatchInput    = "match.input"
	TypeMatchSnapshot = "match.snapshot"
	TypeMatchEnded    = "match.ended"
)

// DispatchFunc is called by the hub for every incoming message that the hub
// does not handle internally (i.e. everything except ping/pong).
// msgType is env.Type; payload is env.Payload (raw JSON).
// It must be safe to call concurrently.
type DispatchFunc func(userID uuid.UUID, msgType string, payload json.RawMessage)

// Envelope is the outer wrapper for every WebSocket message.
//
//	{"v":1,"type":"ping","payload":{}}
type Envelope struct {
	V       int             `json:"v"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// PingPayload is the payload for TypePing messages (currently empty).
type PingPayload struct{}

// PongPayload is the payload for TypePong messages (currently empty).
type PongPayload struct{}

// ErrorPayload is the payload for TypeError messages pushed by the server.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Decode decodes the Envelope's Payload field into dst (a non-nil pointer).
func (e Envelope) Decode(dst any) error {
	return json.Unmarshal(e.Payload, dst)
}

// Marshal builds a JSON-encoded Envelope for the given type and payload.
// payload must be JSON-serialisable.
func Marshal(msgType string, payload any) ([]byte, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ws marshal payload: %w", err)
	}
	env := Envelope{
		V:       Version,
		Type:    msgType,
		Payload: p,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("ws marshal envelope: %w", err)
	}
	return b, nil
}

// Unmarshal decodes raw bytes into an Envelope and then decodes the Payload
// field into dst (which must be a non-nil pointer).
func Unmarshal(data []byte, dst any) (Envelope, error) {
	var env Envelope
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return Envelope{}, fmt.Errorf("ws unmarshal envelope: %w", err)
	}
	if env.V != Version {
		return env, fmt.Errorf("ws unsupported protocol version: %d", env.V)
	}
	if env.Type == "" {
		return env, fmt.Errorf("ws missing message type")
	}
	if dst != nil {
		if err := json.Unmarshal(env.Payload, dst); err != nil {
			return env, fmt.Errorf("ws unmarshal payload (%s): %w", env.Type, err)
		}
	}
	return env, nil
}
