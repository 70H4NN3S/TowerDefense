package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
	"github.com/70H4NN3S/TowerDefense/internal/ws"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeStore struct {
	channels    map[uuid.UUID]Channel
	memberships map[uuid.UUID]map[uuid.UUID]bool // channelID → set of userIDs
	messages    []Message
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		channels:    make(map[uuid.UUID]Channel),
		memberships: make(map[uuid.UUID]map[uuid.UUID]bool),
	}
}

func (f *fakeStore) addChannel(ch Channel) {
	f.channels[ch.ID] = ch
}

func (f *fakeStore) GetChannel(_ context.Context, id uuid.UUID) (Channel, error) {
	ch, ok := f.channels[id]
	if !ok {
		return Channel{}, ErrChannelNotFound
	}
	return ch, nil
}

func (f *fakeStore) EnsureMembership(_ context.Context, channelID, userID uuid.UUID) error {
	if _, ok := f.memberships[channelID]; !ok {
		f.memberships[channelID] = make(map[uuid.UUID]bool)
	}
	f.memberships[channelID][userID] = true
	return nil
}

func (f *fakeStore) IsMember(_ context.Context, channelID, userID uuid.UUID) (bool, error) {
	return f.memberships[channelID][userID], nil
}

func (f *fakeStore) GetMembers(_ context.Context, channelID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for id := range f.memberships[channelID] {
		ids = append(ids, id)
	}
	return ids, nil
}

func (f *fakeStore) InsertMessage(_ context.Context, msg Message) (Message, error) {
	f.messages = append(f.messages, msg)
	return msg, nil
}

func (f *fakeStore) GetHistory(_ context.Context, channelID uuid.UUID, before *time.Time, limit int) ([]Message, error) {
	var out []Message
	for i := len(f.messages) - 1; i >= 0; i-- {
		m := f.messages[i]
		if m.ChannelID != channelID {
			continue
		}
		if before != nil && !m.CreatedAt.Before(*before) {
			continue
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

type fakeHub struct {
	sent []hubSend
}

type hubSend struct {
	userID uuid.UUID
	data   []byte
}

func (h *fakeHub) Send(userID uuid.UUID, data []byte) {
	h.sent = append(h.sent, hubSend{userID: userID, data: data})
}

// ── helpers ───────────────────────────────────────────────────────────────────

func fixedNow() func() time.Time {
	t := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func globalChannel() Channel {
	return Channel{ID: GlobalChannelID, Kind: "global"}
}

func privateChannel(id uuid.UUID) Channel {
	return Channel{ID: id, Kind: "direct"}
}

// ── Send tests ─────────────────────────────────────────────────────────────────

func TestSend_HappyPath_GlobalChannel(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	hub := &fakeHub{}
	svc := NewServiceWithStore(store, hub, fixedNow())

	userID := uuid.New()
	msg, err := svc.Send(context.Background(), GlobalChannelID, userID, "hello")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if msg.Body != "hello" {
		t.Errorf("body = %q, want %q", msg.Body, "hello")
	}
	// auto-join: user must now be a member
	if !store.memberships[GlobalChannelID][userID] {
		t.Error("user was not auto-joined to global channel")
	}
	// broadcast: one send (to the sender themselves, since they're the only member)
	if len(hub.sent) != 1 {
		t.Errorf("hub.sent len = %d, want 1", len(hub.sent))
	}
}

func TestSend_TrimmsLeadingTrailingSpace(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	msg, err := svc.Send(context.Background(), GlobalChannelID, uuid.New(), "  hi  ")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if msg.Body != "hi" {
		t.Errorf("body = %q, want trimmed %q", msg.Body, "hi")
	}
}

func TestSend_ErrorCases(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	privateID := uuid.New()
	store.addChannel(privateChannel(privateID))

	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())
	ctx := context.Background()
	userID := uuid.New()

	tests := []struct {
		name      string
		channelID uuid.UUID
		userID    uuid.UUID
		body      string
		wantErr   error
	}{
		{"empty body", GlobalChannelID, userID, "", ErrBodyEmpty},
		{"whitespace only body", GlobalChannelID, userID, "   ", ErrBodyEmpty},
		{"body too long", GlobalChannelID, userID, strings.Repeat("a", 501), ErrBodyTooLong},
		{"unknown channel", uuid.New(), userID, "hi", ErrChannelNotFound},
		{"not a member of private channel", privateID, userID, "hi", ErrNotMember},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.Send(ctx, tt.channelID, tt.userID, tt.body)
			if err == nil {
				t.Fatalf("err = nil, want %v", tt.wantErr)
			}
			if !isErr(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSend_PrivateChannel_MemberCanSend(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	privateID := uuid.New()
	store.addChannel(privateChannel(privateID))
	userID := uuid.New()
	// pre-add as member
	_ = store.EnsureMembership(context.Background(), privateID, userID)

	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())
	_, err := svc.Send(context.Background(), privateID, userID, "secret")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestSend_Broadcast500CharBody(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	body := strings.Repeat("x", 500)
	_, err := svc.Send(context.Background(), GlobalChannelID, uuid.New(), body)
	if err != nil {
		t.Fatalf("err = %v, want nil for exactly 500 chars", err)
	}
}

// ── History tests ─────────────────────────────────────────────────────────────

func TestHistory_GlobalChannelNoMembershipRequired(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	// Seed three messages.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 3 {
		store.messages = append(store.messages, Message{
			ID:        uuid.New(),
			ChannelID: GlobalChannelID,
			UserID:    uuid.New(),
			Body:      "msg",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}

	msgs, err := svc.History(context.Background(), GlobalChannelID, uuid.New(), nil, 10)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(msgs) != 3 {
		t.Errorf("len = %d, want 3", len(msgs))
	}
}

func TestHistory_LimitClamped(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 10 {
		store.messages = append(store.messages, Message{
			ID:        uuid.New(),
			ChannelID: GlobalChannelID,
			UserID:    uuid.New(),
			Body:      "msg",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}

	msgs, err := svc.History(context.Background(), GlobalChannelID, uuid.New(), nil, 3)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(msgs) != 3 {
		t.Errorf("len = %d, want 3 (limit)", len(msgs))
	}
}

func TestHistory_BeforeCursor(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		store.messages = append(store.messages, Message{
			ID:        uuid.New(),
			ChannelID: GlobalChannelID,
			UserID:    uuid.New(),
			Body:      "msg",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}

	// Request messages before minute 3 → should get messages at minutes 0,1,2.
	cursor := base.Add(3 * time.Minute)
	msgs, err := svc.History(context.Background(), GlobalChannelID, uuid.New(), &cursor, 10)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(msgs) != 3 {
		t.Errorf("len = %d, want 3", len(msgs))
	}
}

func TestHistory_NotMemberOfPrivateChannel(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	privateID := uuid.New()
	store.addChannel(privateChannel(privateID))
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	_, err := svc.History(context.Background(), privateID, uuid.New(), nil, 10)
	if !isErr(err, ErrNotMember) {
		t.Errorf("err = %v, want ErrNotMember", err)
	}
}

// ── EnsureMembership tests ────────────────────────────────────────────────────

func TestEnsureMembership_IdempotentOnRepeat(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())

	userID := uuid.New()
	ctx := context.Background()

	if err := svc.EnsureMembership(ctx, GlobalChannelID, userID); err != nil {
		t.Fatalf("first call err = %v", err)
	}
	if err := svc.EnsureMembership(ctx, GlobalChannelID, userID); err != nil {
		t.Fatalf("second call err = %v", err)
	}
	if len(store.memberships[GlobalChannelID]) != 1 {
		t.Errorf("membership count = %d, want 1", len(store.memberships[GlobalChannelID]))
	}
}

// ── Dispatch / chat.typing tests ─────────────────────────────────────────────

func TestDispatch_ChatTyping_BroadcastsToOtherMembers(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	store.addChannel(globalChannel())
	hub := &fakeHub{}
	svc := NewServiceWithStore(store, hub, fixedNow())

	sender := uuid.New()
	other := uuid.New()
	ctx := context.Background()
	_ = store.EnsureMembership(ctx, GlobalChannelID, sender)
	_ = store.EnsureMembership(ctx, GlobalChannelID, other)

	p, _ := json.Marshal(chatTypingClientPayload{ChannelID: GlobalChannelID.String()})
	svc.Dispatch(sender, ws.TypeChatTyping, p)

	// Should be sent to other but NOT to sender.
	if len(hub.sent) != 1 {
		t.Fatalf("hub.sent len = %d, want 1", len(hub.sent))
	}
	if hub.sent[0].userID != other {
		t.Errorf("recipient = %v, want other (%v)", hub.sent[0].userID, other)
	}

	var env ws.Envelope
	if err := json.Unmarshal(hub.sent[0].data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Type != ws.TypeChatTyping {
		t.Errorf("type = %q, want %q", env.Type, ws.TypeChatTyping)
	}
}

func TestDispatch_ChatTyping_NotMemberDroppedSilently(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	privateID := uuid.New()
	store.addChannel(privateChannel(privateID))
	hub := &fakeHub{}
	svc := NewServiceWithStore(store, hub, fixedNow())

	// Non-member sends typing.
	p, _ := json.Marshal(chatTypingClientPayload{ChannelID: privateID.String()})
	svc.Dispatch(uuid.New(), ws.TypeChatTyping, p)

	if len(hub.sent) != 0 {
		t.Errorf("hub.sent len = %d, want 0 (dropped)", len(hub.sent))
	}
}

func TestDispatch_UnknownType_Logged(t *testing.T) {
	t.Parallel()
	// Should not panic; just logs a warning.
	store := newFakeStore()
	svc := NewServiceWithStore(store, &fakeHub{}, fixedNow())
	svc.Dispatch(uuid.New(), "chat.unknown", nil)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// isErr is a simple errors.Is wrapper for the test-package scope.
func isErr(err, target error) bool {
	if err == nil {
		return target == nil
	}
	s := err.Error()
	return s == target.Error() || containsErrString(s, target.Error())
}

// containsErrString checks wrapped error strings since we use fmt.Errorf("%w").
func containsErrString(chain, target string) bool {
	return len(chain) >= len(target) && (chain == target ||
		len(chain) > len(target) && chain[len(chain)-len(target)-2:] == ": "+target ||
		strings.HasSuffix(chain, ": "+target))
}
