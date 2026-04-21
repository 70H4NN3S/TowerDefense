package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/middleware"
	"github.com/70H4NN3S/TowerDefense/internal/httpserver/respond"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ChatSvc is the domain interface consumed by ChatHandler.
type ChatSvc interface {
	Send(ctx context.Context, channelID, userID uuid.UUID, body string) (chat.Message, error)
	History(ctx context.Context, channelID, userID uuid.UUID, before *time.Time, limit int) ([]chat.Message, error)
}

// ChatHandler wires chat-related routes onto a ServeMux.
type ChatHandler struct {
	svc     ChatSvc
	limiter *chatLimiter
	jwtKey  []byte
}

// NewChatHandler constructs a ChatHandler.
// rate is the maximum number of messages a user can send per channel per window.
func NewChatHandler(svc ChatSvc, jwtKey []byte) *ChatHandler {
	// 10 messages per 10 seconds per user-channel pair.
	return &ChatHandler{
		svc:     svc,
		jwtKey:  jwtKey,
		limiter: newChatLimiter(10, 10*time.Second),
	}
}

// Register wires routes onto mux. All routes require authentication.
func (h *ChatHandler) Register(mux *http.ServeMux) {
	auth := middleware.Authenticate(h.jwtKey)
	mux.Handle("GET /v1/chat/channels/{id}/messages", auth(http.HandlerFunc(h.handleHistory)))
	mux.Handle("POST /v1/chat/channels/{id}/messages", auth(http.HandlerFunc(h.handleSend)))
}

// ── response shapes ───────────────────────────────────────────────────────────

type messageResponse struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func messageToResponse(m chat.Message) messageResponse {
	return messageResponse{
		ID:        m.ID.String(),
		ChannelID: m.ChannelID.String(),
		UserID:    m.UserID.String(),
		Body:      m.Body,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// handleHistory serves GET /v1/chat/channels/{id}/messages.
// Query params: before (RFC 3339 timestamp), limit (1–100, default 50).
func (h *ChatHandler) handleHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, chat.ErrNotMember)
		return
	}

	channelID, ok := parseChannelID(w, r)
	if !ok {
		return
	}

	var before *time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
				Error: respond.ErrorDetail{
					Code:    "invalid_param",
					Message: "before must be an RFC 3339 timestamp.",
				},
			})
			return
		}
		before = &t
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 || n > 100 {
			respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
				Error: respond.ErrorDetail{
					Code:    "invalid_param",
					Message: "limit must be an integer between 1 and 100.",
				},
			})
			return
		}
		limit = n
	}

	msgs, err := h.svc.History(r.Context(), channelID, userID, before, limit)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	out := make([]messageResponse, len(msgs))
	for i, m := range msgs {
		out[i] = messageToResponse(m)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"messages": out})
}

// handleSend serves POST /v1/chat/channels/{id}/messages.
// JSON body: {"body": "..."}
func (h *ChatHandler) handleSend(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.Error(w, r, chat.ErrNotMember)
		return
	}

	channelID, ok := parseChannelID(w, r)
	if !ok {
		return
	}

	if allowed, retryAfter := h.limiter.Allow(userID, channelID); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		respond.JSON(w, http.StatusTooManyRequests, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{
				Code:    "rate_limited",
				Message: "You are sending messages too quickly.",
			},
		})
		return
	}

	var body struct {
		Body string `json:"body"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_body", Message: "Request body is malformed."},
		})
		return
	}

	msg, err := h.svc.Send(r.Context(), channelID, userID, body.Body)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"message": messageToResponse(msg)})
}

// ── shared helpers ─────────────────────────────────────────────────────────────

func parseChannelID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respond.JSON(w, http.StatusBadRequest, respond.ErrorEnvelope{
			Error: respond.ErrorDetail{Code: "invalid_id", Message: "Channel ID is not a valid UUID."},
		})
		return "", false
	}
	return id, true
}

// ── per-user-per-channel rate limiter ─────────────────────────────────────────

// chatKey is the composite key for the per-user-per-channel token bucket.
type chatKey struct {
	userID    uuid.UUID
	channelID uuid.UUID
}

// chatBucket is a token-bucket state for one user-channel pair.
type chatBucket struct {
	tokens    float64
	lastRefil time.Time
}

// chatLimiter is an in-memory per-user-per-channel token-bucket rate limiter.
type chatLimiter struct {
	mu       sync.Mutex
	buckets  map[chatKey]*chatBucket
	rate     float64 // tokens added per second
	capacity float64
	window   time.Duration
}

func newChatLimiter(capacity int, window time.Duration) *chatLimiter {
	return &chatLimiter{
		buckets:  make(map[chatKey]*chatBucket),
		rate:     float64(capacity) / window.Seconds(),
		capacity: float64(capacity),
		window:   window,
	}
}

// Allow reports whether userID can post to channelID. Returns the retry-after
// seconds when denied (0 when allowed).
func (l *chatLimiter) Allow(userID, channelID uuid.UUID) (allowed bool, retryAfterSecs int) {
	key := chatKey{userID: userID, channelID: channelID}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &chatBucket{tokens: l.capacity, lastRefil: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefil).Seconds()
	if elapsed > l.window.Seconds() {
		delete(l.buckets, key)
		return true, 0
	}

	b.tokens = min(l.capacity, b.tokens+elapsed*l.rate)
	b.lastRefil = now

	if b.tokens < 1 {
		need := 1 - b.tokens
		secs := max(1, int(math.Ceil(need/l.rate)))
		return false, secs
	}
	b.tokens--
	return true, 0
}
