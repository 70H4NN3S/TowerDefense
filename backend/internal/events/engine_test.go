package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeStore struct {
	events    []Event
	progress  map[string]Progress // key: "eventID/userID"
	saveErr   error
}

func newFakeStore(events []Event) *fakeStore {
	return &fakeStore{events: events, progress: map[string]Progress{}}
}

func progressKey(eventID, userID uuid.UUID) string {
	return eventID.String() + "/" + userID.String()
}

func (f *fakeStore) ActiveAndUpcoming(_ context.Context, now time.Time) ([]Event, error) {
	var out []Event
	for _, ev := range f.events {
		if ev.StartsAt.Before(now.Add(lookahead)) && ev.EndsAt.After(now) {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (f *fakeStore) GetEvent(_ context.Context, id uuid.UUID) (Event, error) {
	for _, ev := range f.events {
		if ev.ID == id {
			return ev, nil
		}
	}
	return Event{}, ErrEventNotFound
}

func (f *fakeStore) GetOrCreateProgress(_ context.Context, eventID, userID uuid.UUID) (Progress, error) {
	key := progressKey(eventID, userID)
	p, ok := f.progress[key]
	if !ok {
		p = Progress{
			EventID:        eventID,
			UserID:         userID,
			Data:           json.RawMessage("{}"),
			ClaimedRewards: []int{},
		}
	}
	return p, nil
}

func (f *fakeStore) SaveProgress(_ context.Context, p Progress) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.progress[progressKey(p.EventID, p.UserID)] = p
	return nil
}

type fakeAwarder struct {
	gold     map[string]int64
	diamonds map[string]int64
}

func newFakeAwarder() *fakeAwarder {
	return &fakeAwarder{
		gold:     map[string]int64{},
		diamonds: map[string]int64{},
	}
}

func (a *fakeAwarder) AddGold(_ context.Context, id uuid.UUID, n int64) error {
	a.gold[id.String()] += n
	return nil
}

func (a *fakeAwarder) AddDiamonds(_ context.Context, id uuid.UUID, n int64) error {
	a.diamonds[id.String()] += n
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

var fixedNow = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func activeEvent() Event {
	return Event{
		ID:       uuid.New(),
		Kind:     "kill_n_monsters",
		Name:     "Slayer Challenge",
		StartsAt: fixedNow.Add(-time.Hour),
		EndsAt:   fixedNow.Add(24 * time.Hour),
		Config:   json.RawMessage(`{"tiers":[{"threshold":10,"rewards":{"gold":100}},{"threshold":50,"rewards":{"gold":500,"diamonds":5}}]}`),
	}
}

func makeEngine(store Store, awarder Awarder) *Engine {
	return NewEngineWithStore(store, awarder, func() time.Time { return fixedNow })
}

// ── ActiveAndUpcoming tests ───────────────────────────────────────────────────

func TestActiveAndUpcoming_ReturnsActiveEvent(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	events, err := eng.ActiveAndUpcoming(context.Background())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(events) != 1 || events[0].ID != ev.ID {
		t.Errorf("events = %v, want [%s]", events, ev.ID)
	}
}

func TestActiveAndUpcoming_ExcludesExpiredEvent(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	ev.EndsAt = fixedNow.Add(-time.Minute) // already ended
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	events, err := eng.ActiveAndUpcoming(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("len = %d, want 0 (event expired)", len(events))
	}
}

func TestActiveAndUpcoming_IncludesUpcomingWithinLookahead(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	ev.StartsAt = fixedNow.Add(3 * 24 * time.Hour)  // starts in 3 days
	ev.EndsAt = fixedNow.Add(10 * 24 * time.Hour)
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	events, err := eng.ActiveAndUpcoming(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(events) != 1 {
		t.Errorf("len = %d, want 1 (upcoming within lookahead)", len(events))
	}
}

// ── RecordMatchResult tests ───────────────────────────────────────────────────

func TestRecordMatchResult_AccumulatesKillCount(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())
	ctx := context.Background()
	uid := uuid.New()

	if err := eng.RecordMatchResult(ctx, uid, 5, 3, true); err != nil {
		t.Fatalf("err = %v", err)
	}
	if err := eng.RecordMatchResult(ctx, uid, 7, 2, false); err != nil {
		t.Fatalf("err = %v", err)
	}

	p, err := store.GetOrCreateProgress(ctx, ev.ID, uid)
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	val, err := KillNMonsters{}.ProgressValue(p.Data)
	if err != nil {
		t.Fatalf("progress value: %v", err)
	}
	if val != 12 {
		t.Errorf("kill count = %d, want 12", val)
	}
}

func TestRecordMatchResult_SkipsUpcomingEvent(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	ev.StartsAt = fixedNow.Add(time.Hour) // not started yet
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	_ = eng.RecordMatchResult(context.Background(), uuid.New(), 10, 1, true)

	// No progress should have been saved.
	if len(store.progress) != 0 {
		t.Errorf("progress entries = %d, want 0", len(store.progress))
	}
}

// ── ClaimReward tests ─────────────────────────────────────────────────────────

func TestClaimReward_HappyPath(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	awarder := newFakeAwarder()
	eng := makeEngine(store, awarder)
	ctx := context.Background()
	uid := uuid.New()

	// Seed enough kills to reach tier 0 (threshold 10).
	_ = eng.RecordMatchResult(ctx, uid, 15, 0, false)

	rewards, err := eng.ClaimReward(ctx, uid, ev.ID, 0)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if rewards["gold"] != 100 {
		t.Errorf("gold = %d, want 100", rewards["gold"])
	}
	if awarder.gold[uid.String()] != 100 {
		t.Errorf("awarder.gold = %d, want 100", awarder.gold[uid.String()])
	}
}

func TestClaimReward_EventNotFound(t *testing.T) {
	t.Parallel()
	store := newFakeStore(nil)
	eng := makeEngine(store, newFakeAwarder())

	_, err := eng.ClaimReward(context.Background(), uuid.New(), uuid.New(), 0)
	if !errors.Is(err, ErrEventNotFound) {
		t.Errorf("err = %v, want ErrEventNotFound", err)
	}
}

func TestClaimReward_EventNotActive_NotStarted(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	ev.StartsAt = fixedNow.Add(time.Hour)
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	_, err := eng.ClaimReward(context.Background(), uuid.New(), ev.ID, 0)
	if !errors.Is(err, ErrEventNotActive) {
		t.Errorf("err = %v, want ErrEventNotActive", err)
	}
}

func TestClaimReward_EventNotActive_Ended(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	ev.EndsAt = fixedNow.Add(-time.Minute)
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	_, err := eng.ClaimReward(context.Background(), uuid.New(), ev.ID, 0)
	if !errors.Is(err, ErrEventNotActive) {
		t.Errorf("err = %v, want ErrEventNotActive", err)
	}
}

func TestClaimReward_TierInvalid_Negative(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	_, err := eng.ClaimReward(context.Background(), uuid.New(), ev.ID, -1)
	if !errors.Is(err, ErrTierInvalid) {
		t.Errorf("err = %v, want ErrTierInvalid", err)
	}
}

func TestClaimReward_TierInvalid_TooHigh(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())

	_, err := eng.ClaimReward(context.Background(), uuid.New(), ev.ID, 99)
	if !errors.Is(err, ErrTierInvalid) {
		t.Errorf("err = %v, want ErrTierInvalid", err)
	}
}

func TestClaimReward_TierNotReached(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())
	ctx := context.Background()
	uid := uuid.New()

	// Only 3 kills — tier 0 requires 10.
	_ = eng.RecordMatchResult(ctx, uid, 3, 0, false)

	_, err := eng.ClaimReward(ctx, uid, ev.ID, 0)
	if !errors.Is(err, ErrTierNotReached) {
		t.Errorf("err = %v, want ErrTierNotReached", err)
	}
}

func TestClaimReward_TierAlreadyClaimed(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	eng := makeEngine(store, newFakeAwarder())
	ctx := context.Background()
	uid := uuid.New()

	_ = eng.RecordMatchResult(ctx, uid, 15, 0, true)
	if _, err := eng.ClaimReward(ctx, uid, ev.ID, 0); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	_, err := eng.ClaimReward(ctx, uid, ev.ID, 0)
	if !errors.Is(err, ErrTierAlreadyClaimed) {
		t.Errorf("err = %v, want ErrTierAlreadyClaimed", err)
	}
}

func TestClaimReward_MultipleTiersClaimed(t *testing.T) {
	t.Parallel()
	ev := activeEvent()
	store := newFakeStore([]Event{ev})
	awarder := newFakeAwarder()
	eng := makeEngine(store, awarder)
	ctx := context.Background()
	uid := uuid.New()

	// 60 kills reaches both tier 0 (10) and tier 1 (50).
	_ = eng.RecordMatchResult(ctx, uid, 60, 0, true)

	if _, err := eng.ClaimReward(ctx, uid, ev.ID, 0); err != nil {
		t.Fatalf("claim tier 0: %v", err)
	}
	if _, err := eng.ClaimReward(ctx, uid, ev.ID, 1); err != nil {
		t.Fatalf("claim tier 1: %v", err)
	}

	if awarder.gold[uid.String()] != 600 {
		t.Errorf("total gold = %d, want 600 (100+500)", awarder.gold[uid.String()])
	}
	if awarder.diamonds[uid.String()] != 5 {
		t.Errorf("total diamonds = %d, want 5", awarder.diamonds[uid.String()])
	}
}

// ── KillNMonsters unit tests ──────────────────────────────────────────────────

func TestKillNMonsters_UpdateProgress_FirstMatch(t *testing.T) {
	t.Parallel()
	k := KillNMonsters{}
	data, err := k.UpdateProgress(nil, 10, 3, true)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	val, _ := k.ProgressValue(data)
	if val != 10 {
		t.Errorf("count = %d, want 10", val)
	}
}

func TestKillNMonsters_UpdateProgress_Accumulates(t *testing.T) {
	t.Parallel()
	k := KillNMonsters{}
	data, _ := k.UpdateProgress(nil, 5, 0, false)
	data, err := k.UpdateProgress(data, 8, 0, false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	val, _ := k.ProgressValue(data)
	if val != 13 {
		t.Errorf("count = %d, want 13", val)
	}
}

func TestKillNMonsters_ProgressValue_EmptyData(t *testing.T) {
	t.Parallel()
	k := KillNMonsters{}
	val, err := k.ProgressValue(nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if val != 0 {
		t.Errorf("val = %d, want 0", val)
	}
}

func TestKillNMonsters_Tiers_ParsesConfig(t *testing.T) {
	t.Parallel()
	k := KillNMonsters{}
	config := json.RawMessage(`{"tiers":[{"threshold":50,"rewards":{"gold":500}},{"threshold":100,"rewards":{"gold":1000}}]}`)
	tiers, err := k.Tiers(config)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(tiers) != 2 {
		t.Fatalf("len(tiers) = %d, want 2", len(tiers))
	}
	if tiers[0].Threshold != 50 {
		t.Errorf("tier[0].Threshold = %d, want 50", tiers[0].Threshold)
	}
	if tiers[1].Rewards["gold"] != 1000 {
		t.Errorf("tier[1].gold = %d, want 1000", tiers[1].Rewards["gold"])
	}
}
