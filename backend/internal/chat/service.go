package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
	"github.com/70H4NN3S/TowerDefense/internal/ws"
)

// GlobalChannelID is the fixed UUID of the pre-seeded global chat channel.
// It matches the value inserted by migration 0008_chat_global_seed.
var GlobalChannelID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

// maxBody is the maximum allowed length of a chat message body in Unicode
// code points. Enforced here and by a DB CHECK constraint.
const maxBody = 500

// Channel is a chat channel row.
type Channel struct {
	ID        uuid.UUID
	Kind      string // "global" | "alliance" | "direct"
	OwnerID   *uuid.UUID
	CreatedAt time.Time
}

// Message is a persisted chat message.
type Message struct {
	ID        uuid.UUID
	ChannelID uuid.UUID
	UserID    uuid.UUID
	Body      string
	CreatedAt time.Time
}

// ── store interface ────────────────────────────────────────────────────────────

// Store abstracts chat persistence. Declared consumer-side so tests can use fakes.
type Store interface {
	// GetChannel returns the channel with the given ID.
	GetChannel(ctx context.Context, id uuid.UUID) (Channel, error)
	// InsertChannel persists a new channel row and returns it.
	InsertChannel(ctx context.Context, ch Channel) (Channel, error)
	// DeleteChannel removes the channel and cascades to memberships and messages.
	DeleteChannel(ctx context.Context, channelID uuid.UUID) error
	// EnsureMembership upserts (channel_id, user_id) into chat_memberships.
	EnsureMembership(ctx context.Context, channelID, userID uuid.UUID) error
	// DeleteMembership removes userID from channelID.
	DeleteMembership(ctx context.Context, channelID, userID uuid.UUID) error
	// IsMember reports whether userID is a member of channelID.
	IsMember(ctx context.Context, channelID, userID uuid.UUID) (bool, error)
	// GetMembers returns all user IDs that are members of channelID.
	GetMembers(ctx context.Context, channelID uuid.UUID) ([]uuid.UUID, error)
	// InsertMessage persists a new message and returns it with its DB-assigned
	// created_at timestamp.
	InsertMessage(ctx context.Context, msg Message) (Message, error)
	// GetHistory returns up to limit messages in channelID with created_at
	// strictly before the before cursor, ordered newest-first.
	// When before is nil, returns the most recent messages.
	GetHistory(ctx context.Context, channelID uuid.UUID, before *time.Time, limit int) ([]Message, error)
}

// ── hub interface ──────────────────────────────────────────────────────────────

// Hub is the subset of ws.Hub used by the chat service to push messages.
type Hub interface {
	Send(userID uuid.UUID, data []byte)
}

// ── service ────────────────────────────────────────────────────────────────────

// Service implements chat: sending messages, fetching history, and managing
// channel memberships.
type Service struct {
	store Store
	hub   Hub
	now   func() time.Time
}

// NewService constructs a Service backed by pool and hub.
func NewService(pool *pgxpool.Pool, hub Hub) *Service {
	return &Service{
		store: newStore(pool),
		hub:   hub,
		now:   time.Now,
	}
}

// NewServiceWithStore constructs a Service from explicit dependencies.
// Intended for tests.
func NewServiceWithStore(store Store, hub Hub, now func() time.Time) *Service {
	return &Service{store: store, hub: hub, now: now}
}

// CreateChannel inserts a new channel of the given kind and returns it.
// ownerID is optional (nil for non-user-owned channels).
func (s *Service) CreateChannel(ctx context.Context, kind string, ownerID *uuid.UUID) (Channel, error) {
	ch := Channel{
		ID:        uuid.New(),
		Kind:      kind,
		OwnerID:   ownerID,
		CreatedAt: s.now(),
	}
	ch, err := s.store.InsertChannel(ctx, ch)
	if err != nil {
		return Channel{}, fmt.Errorf("create channel: %w", err)
	}
	return ch, nil
}

// DeleteChannel removes channelID and all its memberships and messages.
func (s *Service) DeleteChannel(ctx context.Context, channelID uuid.UUID) error {
	if err := s.store.DeleteChannel(ctx, channelID); err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}

// EnsureMembership adds userID to channelID if not already a member.
// For global channels this is always a no-op-or-join; for other kinds callers
// are expected to verify authorization before calling.
func (s *Service) EnsureMembership(ctx context.Context, channelID, userID uuid.UUID) error {
	if err := s.store.EnsureMembership(ctx, channelID, userID); err != nil {
		return fmt.Errorf("ensure membership: %w", err)
	}
	return nil
}

// RemoveMembership removes userID from channelID. It is a no-op if the user
// is not a member.
func (s *Service) RemoveMembership(ctx context.Context, channelID, userID uuid.UUID) error {
	if err := s.store.DeleteMembership(ctx, channelID, userID); err != nil {
		return fmt.Errorf("remove membership: %w", err)
	}
	return nil
}

// Send validates body, persists the message, and broadcasts it to all channel
// members over the WS hub.
//
// For global channels the sender is automatically added as a member before
// the message is inserted. For other channel kinds the caller must already be
// a member or Send returns ErrNotMember.
func (s *Service) Send(ctx context.Context, channelID, userID uuid.UUID, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Message{}, ErrBodyEmpty
	}
	if utf8.RuneCountInString(body) > maxBody {
		return Message{}, ErrBodyTooLong
	}

	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		return Message{}, fmt.Errorf("send: %w", err)
	}

	if ch.Kind == "global" {
		// Auto-join: anyone can write to the global channel.
		if err := s.store.EnsureMembership(ctx, channelID, userID); err != nil {
			return Message{}, fmt.Errorf("send: auto-join: %w", err)
		}
	} else {
		ok, err := s.store.IsMember(ctx, channelID, userID)
		if err != nil {
			return Message{}, fmt.Errorf("send: check membership: %w", err)
		}
		if !ok {
			return Message{}, ErrNotMember
		}
	}

	msg := Message{
		ID:        uuid.New(),
		ChannelID: channelID,
		UserID:    userID,
		Body:      body,
		CreatedAt: s.now(),
	}
	msg, err = s.store.InsertMessage(ctx, msg)
	if err != nil {
		return Message{}, fmt.Errorf("send: persist: %w", err)
	}

	s.broadcastMessage(ctx, msg)
	return msg, nil
}

// History returns up to limit messages from channelID, ordered newest-first.
// before is an optional exclusive cursor (ISO-8601 timestamp). limit is
// clamped to [1, 100].
//
// The requesting user must be a member of the channel, unless it is global.
func (s *Service) History(ctx context.Context, channelID, userID uuid.UUID, before *time.Time, limit int) ([]Message, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}

	if ch.Kind != "global" {
		ok, err := s.store.IsMember(ctx, channelID, userID)
		if err != nil {
			return nil, fmt.Errorf("history: check membership: %w", err)
		}
		if !ok {
			return nil, ErrNotMember
		}
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	msgs, err := s.store.GetHistory(ctx, channelID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	return msgs, nil
}

// Dispatch handles incoming WS messages with the "chat." prefix.
// Currently handles chat.typing: validates membership and re-broadcasts the
// typing indicator to all channel members.
func (s *Service) Dispatch(userID uuid.UUID, msgType string, payload json.RawMessage) {
	switch msgType {
	case ws.TypeChatTyping:
		s.handleTyping(userID, payload)
	default:
		slog.Warn("chat: unknown message type", "type", msgType, "user_id", userID)
	}
}

// ── internal helpers ──────────────────────────────────────────────────────────

// broadcastMessage pushes a chat.message WS envelope to every member of the
// message's channel. Errors are logged and not returned; a send failure for one
// member must not prevent delivery to others.
func (s *Service) broadcastMessage(ctx context.Context, msg Message) {
	members, err := s.store.GetMembers(ctx, msg.ChannelID)
	if err != nil {
		slog.Error("chat: get members for broadcast", "err", err, "channel_id", msg.ChannelID)
		return
	}

	data, err := ws.Marshal(ws.TypeChatMessage, chatMessagePayload{
		ChannelID: msg.ChannelID.String(),
		Message:   messageToPayload(msg),
	})
	if err != nil {
		slog.Error("chat: marshal chat.message", "err", err)
		return
	}

	for _, memberID := range members {
		s.hub.Send(memberID, data)
	}
}

// handleTyping broadcasts a chat.typing indicator to all channel members.
func (s *Service) handleTyping(userID uuid.UUID, payload json.RawMessage) {
	var p chatTypingClientPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		slog.Warn("chat: malformed chat.typing payload", "err", err, "user_id", userID)
		return
	}
	channelID, err := uuid.Parse(p.ChannelID)
	if err != nil {
		slog.Warn("chat: invalid channel_id in chat.typing", "err", err, "user_id", userID)
		return
	}

	// Use a background context: this runs outside an HTTP request.
	ctx := context.Background()

	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		slog.Warn("chat: chat.typing for unknown channel", "channel_id", channelID, "user_id", userID)
		return
	}
	if ch.Kind != "global" {
		ok, err := s.store.IsMember(ctx, channelID, userID)
		if err != nil || !ok {
			return // silently drop — not a member
		}
	}

	members, err := s.store.GetMembers(ctx, channelID)
	if err != nil {
		slog.Error("chat: get members for typing broadcast", "err", err, "channel_id", channelID)
		return
	}

	data, err := ws.Marshal(ws.TypeChatTyping, chatTypingServerPayload{
		ChannelID: channelID.String(),
		UserID:    userID.String(),
	})
	if err != nil {
		slog.Error("chat: marshal chat.typing", "err", err)
		return
	}

	for _, memberID := range members {
		if memberID == userID {
			continue // don't echo back to sender
		}
		s.hub.Send(memberID, data)
	}
}

// ── WS payload shapes ─────────────────────────────────────────────────────────

// chatMessagePayload is the payload of a server-pushed chat.message WS message.
type chatMessagePayload struct {
	ChannelID string         `json:"channel_id"`
	Message   messagePayload `json:"message"`
}

// messagePayload is the JSON representation of a single chat message.
type messagePayload struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func messageToPayload(m Message) messagePayload {
	return messagePayload{
		ID:        m.ID.String(),
		ChannelID: m.ChannelID.String(),
		UserID:    m.UserID.String(),
		Body:      m.Body,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// chatTypingClientPayload is what the client sends in a chat.typing message.
type chatTypingClientPayload struct {
	ChannelID string `json:"channel_id"`
}

// chatTypingServerPayload is what the server broadcasts for chat.typing.
type chatTypingServerPayload struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// ── DB store implementation ───────────────────────────────────────────────────

type pgStore struct {
	pool *pgxpool.Pool
}

func newStore(pool *pgxpool.Pool) *pgStore {
	return &pgStore{pool: pool}
}

func (s *pgStore) GetChannel(ctx context.Context, id uuid.UUID) (Channel, error) {
	const q = `
		SELECT id::text, kind, owner_id::text, created_at
		FROM   chat_channels
		WHERE  id = $1::uuid`

	var ch Channel
	var idStr string
	var ownerStr *string
	err := s.pool.QueryRow(ctx, q, id.String()).Scan(&idStr, &ch.Kind, &ownerStr, &ch.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Channel{}, ErrChannelNotFound
	}
	if err != nil {
		return Channel{}, fmt.Errorf("get channel: %w", err)
	}
	ch.ID, err = uuid.Parse(idStr)
	if err != nil {
		return Channel{}, fmt.Errorf("parse channel id: %w", err)
	}
	if ownerStr != nil {
		oid, err := uuid.Parse(*ownerStr)
		if err != nil {
			return Channel{}, fmt.Errorf("parse owner id: %w", err)
		}
		ch.OwnerID = &oid
	}
	return ch, nil
}

// uuidPtrToStr converts a *uuid.UUID to *string for nullable UUID parameters.
// pgx accepts *string as a nullable UUID column value.
func uuidPtrToStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func (s *pgStore) InsertChannel(ctx context.Context, ch Channel) (Channel, error) {
	const q = `
		INSERT INTO chat_channels (id, kind, owner_id, created_at)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, kind, owner_id::text, created_at`

	var idStr string
	var ownerStr *string
	err := s.pool.QueryRow(ctx, q,
		ch.ID.String(), ch.Kind, uuidPtrToStr(ch.OwnerID), ch.CreatedAt,
	).Scan(&idStr, &ch.Kind, &ownerStr, &ch.CreatedAt)
	if err != nil {
		return Channel{}, fmt.Errorf("insert channel: %w", err)
	}
	ch.ID, err = uuid.Parse(idStr)
	if err != nil {
		return Channel{}, fmt.Errorf("parse channel id: %w", err)
	}
	if ownerStr != nil {
		oid, err := uuid.Parse(*ownerStr)
		if err != nil {
			return Channel{}, fmt.Errorf("parse owner id: %w", err)
		}
		ch.OwnerID = &oid
	}
	return ch, nil
}

func (s *pgStore) DeleteChannel(ctx context.Context, channelID uuid.UUID) error {
	const q = `DELETE FROM chat_channels WHERE id = $1::uuid`
	_, err := s.pool.Exec(ctx, q, channelID.String())
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}

func (s *pgStore) EnsureMembership(ctx context.Context, channelID, userID uuid.UUID) error {
	const q = `
		INSERT INTO chat_memberships (channel_id, user_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT (channel_id, user_id) DO NOTHING`

	_, err := s.pool.Exec(ctx, q, channelID.String(), userID.String())
	if err != nil {
		return fmt.Errorf("ensure membership: %w", err)
	}
	return nil
}

func (s *pgStore) DeleteMembership(ctx context.Context, channelID, userID uuid.UUID) error {
	const q = `
		DELETE FROM chat_memberships
		WHERE channel_id = $1::uuid AND user_id = $2::uuid`

	_, err := s.pool.Exec(ctx, q, channelID.String(), userID.String())
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	return nil
}

func (s *pgStore) IsMember(ctx context.Context, channelID, userID uuid.UUID) (bool, error) {
	const q = `
		SELECT 1
		FROM   chat_memberships
		WHERE  channel_id = $1::uuid AND user_id = $2::uuid`

	var dummy int
	err := s.pool.QueryRow(ctx, q, channelID.String(), userID.String()).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is member: %w", err)
	}
	return true, nil
}

func (s *pgStore) GetMembers(ctx context.Context, channelID uuid.UUID) ([]uuid.UUID, error) {
	const q = `
		SELECT user_id::text
		FROM   chat_memberships
		WHERE  channel_id = $1::uuid`

	rows, err := s.pool.Query(ctx, q, channelID.String())
	if err != nil {
		return nil, fmt.Errorf("get members: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("parse member id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *pgStore) InsertMessage(ctx context.Context, msg Message) (Message, error) {
	const q = `
		INSERT INTO chat_messages (id, channel_id, user_id, body, created_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
		RETURNING id::text, channel_id::text, user_id::text, body, created_at`

	var idStr, chStr, uStr string
	err := s.pool.QueryRow(ctx, q,
		msg.ID.String(), msg.ChannelID.String(), msg.UserID.String(), msg.Body, msg.CreatedAt,
	).Scan(&idStr, &chStr, &uStr, &msg.Body, &msg.CreatedAt)
	if err != nil {
		return Message{}, fmt.Errorf("insert message: %w", err)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return Message{}, fmt.Errorf("parse message id: %w", err)
	}
	msg.ID = id

	ch, err := uuid.Parse(chStr)
	if err != nil {
		return Message{}, fmt.Errorf("parse channel id: %w", err)
	}
	msg.ChannelID = ch

	u, err := uuid.Parse(uStr)
	if err != nil {
		return Message{}, fmt.Errorf("parse user id: %w", err)
	}
	msg.UserID = u

	return msg, nil
}

func (s *pgStore) GetHistory(ctx context.Context, channelID uuid.UUID, before *time.Time, limit int) ([]Message, error) {
	const q = `
		SELECT id::text, channel_id::text, user_id::text, body, created_at
		FROM   chat_messages
		WHERE  channel_id = $1::uuid
		  AND  ($2::timestamptz IS NULL OR created_at < $2)
		ORDER  BY created_at DESC
		LIMIT  $3`

	rows, err := s.pool.Query(ctx, q, channelID.String(), before, limit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var idStr, chStr, uStr string
		if err := rows.Scan(&idStr, &chStr, &uStr, &m.Body, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.ID, err = uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("parse message id: %w", err)
		}
		m.ChannelID, err = uuid.Parse(chStr)
		if err != nil {
			return nil, fmt.Errorf("parse channel id: %w", err)
		}
		m.UserID, err = uuid.Parse(uStr)
		if err != nil {
			return nil, fmt.Errorf("parse user id: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
