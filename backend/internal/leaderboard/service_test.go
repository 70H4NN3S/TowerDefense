package leaderboard

import (
	"context"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fake store ────────────────────────────────────────────────────────────────

type fakeStore struct {
	globalEntries  []GlobalEntry
	allianceEntries []AllianceEntry
	memberEntries  []MemberEntry
	refreshGlobal  int
	refreshAlliance int
}

func (f *fakeStore) GlobalPage(_ context.Context, afterRank int64, limit int) ([]GlobalEntry, error) {
	var out []GlobalEntry
	for _, e := range f.globalEntries {
		if e.Rank > afterRank {
			out = append(out, e)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) AlliancePage(_ context.Context, afterTrophies int64, afterID uuid.UUID, limit int, firstPage bool) ([]AllianceEntry, error) {
	var out []AllianceEntry
	for _, e := range f.allianceEntries {
		if firstPage || e.TotalTrophies < afterTrophies ||
			(e.TotalTrophies == afterTrophies && e.AllianceID.String() < afterID.String()) {
			out = append(out, e)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) AllianceMembers(_ context.Context, _ uuid.UUID) ([]MemberEntry, error) {
	return append([]MemberEntry(nil), f.memberEntries...), nil
}

func (f *fakeStore) RefreshGlobal(_ context.Context) error {
	f.refreshGlobal++
	return nil
}

func (f *fakeStore) RefreshAlliance(_ context.Context) error {
	f.refreshAlliance++
	return nil
}

// ── GlobalLeaderboard tests ───────────────────────────────────────────────────

func TestGlobalLeaderboard_FirstPage(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		globalEntries: []GlobalEntry{
			{Rank: 1, UserID: uuid.New(), Trophies: 1000},
			{Rank: 2, UserID: uuid.New(), Trophies: 900},
			{Rank: 3, UserID: uuid.New(), Trophies: 800},
		},
	}
	svc := NewServiceWithStore(store)

	entries, err := svc.GlobalLeaderboard(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 3 {
		t.Errorf("len = %d, want 3", len(entries))
	}
}

func TestGlobalLeaderboard_CursorPagination(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		globalEntries: func() []GlobalEntry {
			out := make([]GlobalEntry, 10)
			for i := range 10 {
				out[i] = GlobalEntry{Rank: int64(i + 1), UserID: uuid.New(), Trophies: int64(1000 - i*100)}
			}
			return out
		}(),
	}
	svc := NewServiceWithStore(store)
	ctx := context.Background()

	// First page: ranks 1–3.
	page1, err := svc.GlobalLeaderboard(ctx, 0, 3)
	if err != nil {
		t.Fatalf("page1 err = %v", err)
	}
	if len(page1) != 3 {
		t.Fatalf("page1 len = %d, want 3", len(page1))
	}
	if page1[0].Rank != 1 || page1[2].Rank != 3 {
		t.Errorf("page1 ranks = [%d..%d], want [1..3]", page1[0].Rank, page1[2].Rank)
	}

	// Second page: ranks 4–6 (cursor = last rank on page1).
	page2, err := svc.GlobalLeaderboard(ctx, page1[len(page1)-1].Rank, 3)
	if err != nil {
		t.Fatalf("page2 err = %v", err)
	}
	if len(page2) != 3 {
		t.Fatalf("page2 len = %d, want 3", len(page2))
	}
	if page2[0].Rank != 4 {
		t.Errorf("page2[0].rank = %d, want 4", page2[0].Rank)
	}
}

func TestGlobalLeaderboard_LimitClamped(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		globalEntries: func() []GlobalEntry {
			out := make([]GlobalEntry, 150)
			for i := range 150 {
				out[i] = GlobalEntry{Rank: int64(i + 1), UserID: uuid.New()}
			}
			return out
		}(),
	}
	svc := NewServiceWithStore(store)

	// Request 200 → clamped to 100.
	entries, err := svc.GlobalLeaderboard(context.Background(), 0, 200)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(entries) != 100 {
		t.Errorf("len = %d, want 100 (clamped)", len(entries))
	}
}

func TestGlobalLeaderboard_DefaultLimitWhenZero(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		globalEntries: func() []GlobalEntry {
			out := make([]GlobalEntry, 50)
			for i := range 50 {
				out[i] = GlobalEntry{Rank: int64(i + 1), UserID: uuid.New()}
			}
			return out
		}(),
	}
	svc := NewServiceWithStore(store)

	// limit=0 → default 25.
	entries, err := svc.GlobalLeaderboard(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(entries) != 25 {
		t.Errorf("len = %d, want 25 (default)", len(entries))
	}
}

func TestGlobalLeaderboard_EmptyView(t *testing.T) {
	t.Parallel()

	svc := NewServiceWithStore(&fakeStore{})
	entries, err := svc.GlobalLeaderboard(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Errorf("len = %d, want 0", len(entries))
	}
}

// ── AllianceLeaderboard tests ─────────────────────────────────────────────────

func TestAllianceLeaderboard_FirstPage(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		allianceEntries: []AllianceEntry{
			{AllianceID: uuid.New(), AllianceName: "Alpha", AllianceTag: "ALP", TotalTrophies: 5000, MemberCount: 10},
			{AllianceID: uuid.New(), AllianceName: "Beta", AllianceTag: "BET", TotalTrophies: 4000, MemberCount: 8},
		},
	}
	svc := NewServiceWithStore(store)

	entries, err := svc.AllianceLeaderboard(context.Background(), -1, uuid.UUID(""), 10, true)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}
	if entries[0].AllianceName != "Alpha" {
		t.Errorf("first alliance = %q, want Alpha", entries[0].AllianceName)
	}
}

func TestAllianceLeaderboard_CursorPagination(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		allianceEntries: []AllianceEntry{
			{AllianceID: uuid.New(), TotalTrophies: 5000},
			{AllianceID: uuid.New(), TotalTrophies: 4000},
			{AllianceID: uuid.New(), TotalTrophies: 3000},
			{AllianceID: uuid.New(), TotalTrophies: 2000},
		},
	}
	svc := NewServiceWithStore(store)
	ctx := context.Background()

	page1, err := svc.AllianceLeaderboard(ctx, -1, uuid.UUID(""), 2, true)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}

	last := page1[len(page1)-1]
	page2, err := svc.AllianceLeaderboard(ctx, last.TotalTrophies, last.AllianceID, 2, false)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}
}

// ── AllianceMemberLeaderboard tests ──────────────────────────────────────────

func TestAllianceMemberLeaderboard_HappyPath(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		memberEntries: []MemberEntry{
			{Rank: 1, UserID: uuid.New(), Role: "leader", Trophies: 1000},
			{Rank: 2, UserID: uuid.New(), Role: "member", Trophies: 800},
		},
	}
	svc := NewServiceWithStore(store)

	entries, err := svc.AllianceMemberLeaderboard(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 2 {
		t.Errorf("len = %d, want 2", len(entries))
	}
	if entries[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", entries[0].Rank)
	}
}

func TestAllianceMemberLeaderboard_EmptyAlliance(t *testing.T) {
	t.Parallel()

	svc := NewServiceWithStore(&fakeStore{})
	entries, err := svc.AllianceMemberLeaderboard(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Errorf("len = %d, want 0", len(entries))
	}
}

// ── ranking stability tests ───────────────────────────────────────────────────

func TestGlobalLeaderboard_RankStabilityAcrossPages(t *testing.T) {
	t.Parallel()

	// 6 entries with distinct ranks; pagination must not skip or repeat any.
	store := &fakeStore{
		globalEntries: []GlobalEntry{
			{Rank: 1, UserID: uuid.New(), Trophies: 600},
			{Rank: 2, UserID: uuid.New(), Trophies: 500},
			{Rank: 3, UserID: uuid.New(), Trophies: 400},
			{Rank: 4, UserID: uuid.New(), Trophies: 300},
			{Rank: 5, UserID: uuid.New(), Trophies: 200},
			{Rank: 6, UserID: uuid.New(), Trophies: 100},
		},
	}
	svc := NewServiceWithStore(store)
	ctx := context.Background()

	seen := make(map[int64]bool)
	var cursor int64
	for {
		page, err := svc.GlobalLeaderboard(ctx, cursor, 2)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			if seen[e.Rank] {
				t.Errorf("rank %d appeared twice — pagination is not stable", e.Rank)
			}
			seen[e.Rank] = true
		}
		cursor = page[len(page)-1].Rank
	}
	if len(seen) != 6 {
		t.Errorf("total seen = %d, want 6", len(seen))
	}
}
