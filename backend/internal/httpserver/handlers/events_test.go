package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/events"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fake event service ────────────────────────────────────────────────────────

type fakeEventSvc struct {
	evs      []events.Event
	listErr  error
	claimErr error
	rewards  map[string]int64
}

func (f *fakeEventSvc) ActiveAndUpcoming(_ context.Context) ([]events.Event, error) {
	return f.evs, f.listErr
}

func (f *fakeEventSvc) ClaimReward(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ int) (map[string]int64, error) {
	return f.rewards, f.claimErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newEventMux(svc *fakeEventSvc) *http.ServeMux {
	mux := http.NewServeMux()
	NewEventHandler(svc, testSecret).Register(mux)
	return mux
}

var fixedEventWindow = struct {
	starts, ends time.Time
}{
	starts: time.Now().Add(-time.Hour),
	ends:   time.Now().Add(24 * time.Hour),
}

func sampleEvent() events.Event {
	return events.Event{
		ID:          uuid.New(),
		Kind:        "kill_n_monsters",
		Name:        "Slayer Challenge",
		Description: "Kill 100 monsters",
		StartsAt:    fixedEventWindow.starts,
		EndsAt:      fixedEventWindow.ends,
		Config:      json.RawMessage(`{"tiers":[{"threshold":10,"rewards":{"gold":100}}]}`),
	}
}

// ── GET /v1/events ────────────────────────────────────────────────────────────

func TestEventList_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	w := doRequest(mux, http.MethodGet, "/v1/events", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestEventList_Empty_Returns200WithEmptySlice(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/events", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["events"]; !ok {
		t.Error("response missing 'events' key")
	}
}

func TestEventList_WithEvents_ReturnsEventList(t *testing.T) {
	t.Parallel()
	ev := sampleEvent()
	svc := &fakeEventSvc{evs: []events.Event{ev}}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/events", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Events []struct {
			ID   string          `json:"id"`
			Kind string          `json:"kind"`
			Name string          `json:"name"`
			Cfg  json.RawMessage `json:"config"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(resp.Events))
	}
	if resp.Events[0].ID != ev.ID.String() {
		t.Errorf("id = %q, want %q", resp.Events[0].ID, ev.ID.String())
	}
	if resp.Events[0].Kind != "kill_n_monsters" {
		t.Errorf("kind = %q, want kill_n_monsters", resp.Events[0].Kind)
	}
}

// ── POST /v1/events/{id}/claim ────────────────────────────────────────────────

func claimPath(id uuid.UUID) string {
	return fmt.Sprintf("/v1/events/%s/claim", id)
}

func TestEventClaim_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), "", `{"tier":0}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestEventClaim_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/events/not-a-uuid/claim", tok, `{"tier":0}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEventClaim_MissingBody_Returns400(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEventClaim_MissingTierField_Returns400(t *testing.T) {
	t.Parallel()
	mux := newEventMux(&fakeEventSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestEventClaim_HappyPath_Returns200WithRewards(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{
		rewards: map[string]int64{"gold": 100},
	}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":0}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Rewards map[string]any `json:"rewards"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Rewards["gold"] == nil {
		t.Error("rewards missing gold")
	}
}

func TestEventClaim_EventNotFound_Returns404(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{claimErr: events.ErrEventNotFound}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":0}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestEventClaim_EventNotActive_Returns409(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{claimErr: events.ErrEventNotActive}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":0}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestEventClaim_TierNotReached_Returns409(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{claimErr: events.ErrTierNotReached}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":0}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestEventClaim_TierAlreadyClaimed_Returns409(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{claimErr: events.ErrTierAlreadyClaimed}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":0}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestEventClaim_TierInvalid_Returns400(t *testing.T) {
	t.Parallel()
	svc := &fakeEventSvc{claimErr: events.ErrTierInvalid}
	mux := newEventMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, claimPath(uuid.New()), tok, `{"tier":999}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
