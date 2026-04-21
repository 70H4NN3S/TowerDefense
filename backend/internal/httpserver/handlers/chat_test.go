package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeChatService ───────────────────────────────────────────────────────────

type fakeChatService struct {
	messages []chat.Message
	sendErr  error
	histErr  error
}

func (f *fakeChatService) Send(_ context.Context, channelID, userID uuid.UUID, body string) (chat.Message, error) {
	if f.sendErr != nil {
		return chat.Message{}, f.sendErr
	}
	m := chat.Message{
		ID:        uuid.New(),
		ChannelID: channelID,
		UserID:    userID,
		Body:      body,
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	f.messages = append(f.messages, m)
	return m, nil
}

func (f *fakeChatService) History(_ context.Context, channelID, userID uuid.UUID, before *time.Time, limit int) ([]chat.Message, error) {
	if f.histErr != nil {
		return nil, f.histErr
	}
	var out []chat.Message
	for _, m := range f.messages {
		if m.ChannelID == channelID {
			out = append(out, m)
		}
	}
	return out, nil
}

// ── test setup ────────────────────────────────────────────────────────────────

var chatChannelID = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

func newChatMux(svc *fakeChatService) *http.ServeMux {
	mux := http.NewServeMux()
	NewChatHandler(svc, testSecret).Register(mux)
	return mux
}

func chatPath(channelID uuid.UUID) string {
	return fmt.Sprintf("/v1/chat/channels/%s/messages", channelID)
}

// ── GET history ───────────────────────────────────────────────────────────────

func TestChatHistory_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	w := doRequest(mux, http.MethodGet, chatPath(chatChannelID), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestChatHistory_InvalidChannelUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/chat/channels/not-a-uuid/messages", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestChatHistory_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{}
	userID := uuid.New()
	svc.messages = append(svc.messages, chat.Message{
		ID:        uuid.New(),
		ChannelID: chatChannelID,
		UserID:    userID,
		Body:      "hello",
		CreatedAt: time.Now(),
	})

	mux := newChatMux(svc)
	tok := signedToken(t, userID)
	w := doRequest(mux, http.MethodGet, chatPath(chatChannelID), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	type resp struct {
		Messages []messageResponse `json:"messages"`
	}
	r := decodeResponse[resp](t, w)
	if len(r.Messages) != 1 {
		t.Errorf("len = %d, want 1", len(r.Messages))
	}
	if r.Messages[0].Body != "hello" {
		t.Errorf("body = %q, want hello", r.Messages[0].Body)
	}
}

func TestChatHistory_InvalidBeforeParam_Returns400(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, chatPath(chatChannelID)+"?before=not-a-time", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestChatHistory_InvalidLimitParam_Returns400(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	tok := signedToken(t, uuid.New())

	tests := []string{"0", "-1", "101", "abc"}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			w := doRequest(mux, http.MethodGet, chatPath(chatChannelID)+"?limit="+raw, tok, "")
			if w.Code != http.StatusBadRequest {
				t.Errorf("limit=%q: status = %d, want 400", raw, w.Code)
			}
		})
	}
}

func TestChatHistory_NotMember_Returns403(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{histErr: chat.ErrNotMember}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, chatPath(chatChannelID), tok, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// ── POST send ─────────────────────────────────────────────────────────────────

func TestChatSend_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), "", `{"body":"hi"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestChatSend_InvalidChannelUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/chat/channels/bad-uuid/messages", tok, `{"body":"hi"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestChatSend_HappyPath_Returns201(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `{"body":"hello world"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}

	type resp struct {
		Message messageResponse `json:"message"`
	}
	r := decodeResponse[resp](t, w)
	if r.Message.Body != "hello world" {
		t.Errorf("body = %q, want %q", r.Message.Body, "hello world")
	}
	if len(svc.messages) != 1 {
		t.Errorf("service message count = %d, want 1", len(svc.messages))
	}
}

func TestChatSend_MalformedBody_Returns400(t *testing.T) {
	t.Parallel()
	mux := newChatMux(&fakeChatService{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestChatSend_EmptyBody_Returns422(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{sendErr: chat.ErrBodyEmpty}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `{"body":""}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestChatSend_BodyTooLong_Returns422(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{sendErr: chat.ErrBodyTooLong}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `{"body":"x"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestChatSend_NotMember_Returns403(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{sendErr: chat.ErrNotMember}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `{"body":"hi"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestChatSend_ChannelNotFound_Returns404(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{sendErr: chat.ErrChannelNotFound}
	mux := newChatMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, chatPath(chatChannelID), tok, `{"body":"hi"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// ── rate limiting ─────────────────────────────────────────────────────────────

func TestChatSend_RateLimitedAfterBurst(t *testing.T) {
	t.Parallel()
	svc := &fakeChatService{}
	mux := http.NewServeMux()
	// Use a tight limiter: 2 messages per 10 seconds.
	h := &ChatHandler{svc: svc, jwtKey: testSecret, limiter: newChatLimiter(2, 10*time.Second)}
	h.Register(mux)

	userID := uuid.New()
	tok := signedToken(t, userID)
	path := chatPath(chatChannelID)

	for i := range 2 {
		w := doRequest(mux, http.MethodPost, path, tok, `{"body":"hi"}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("request %d: status = %d, want 201", i+1, w.Code)
		}
	}

	// Third request should be rate-limited.
	w := doRequest(mux, http.MethodPost, path, tok, `{"body":"hi"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}
}
