package alliance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/models"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakes ─────────────────────────────────────────────────────────────────────

type fakeStore struct {
	alliances map[uuid.UUID]Alliance
	members   map[uuid.UUID]Member  // keyed by userID
	invites   map[uuid.UUID]Invite
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		alliances: make(map[uuid.UUID]Alliance),
		members:   make(map[uuid.UUID]Member),
		invites:   make(map[uuid.UUID]Invite),
	}
}

func (f *fakeStore) InsertAlliance(_ context.Context, a Alliance) (Alliance, error) {
	for _, existing := range f.alliances {
		if existing.Name == a.Name {
			return Alliance{}, ErrNameTaken
		}
		if existing.Tag == a.Tag {
			return Alliance{}, ErrTagTaken
		}
	}
	f.alliances[a.ID] = a
	return a, nil
}

func (f *fakeStore) GetAlliance(_ context.Context, id uuid.UUID) (Alliance, error) {
	a, ok := f.alliances[id]
	if !ok {
		return Alliance{}, ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) SetAllianceLeader(_ context.Context, allianceID, newLeaderID uuid.UUID) error {
	a, ok := f.alliances[allianceID]
	if !ok {
		return ErrNotFound
	}
	a.LeaderID = newLeaderID
	f.alliances[allianceID] = a
	return nil
}

func (f *fakeStore) DeleteAlliance(_ context.Context, id uuid.UUID) error {
	delete(f.alliances, id)
	// Cascade: remove all members belonging to this alliance.
	for uid, m := range f.members {
		if m.AllianceID == id {
			delete(f.members, uid)
		}
	}
	// Cascade: remove all invites for this alliance.
	for iid, inv := range f.invites {
		if inv.AllianceID == id {
			delete(f.invites, iid)
		}
	}
	return nil
}

func (f *fakeStore) InsertMember(_ context.Context, m Member) (Member, error) {
	f.members[m.UserID] = m
	return m, nil
}

func (f *fakeStore) GetMember(_ context.Context, userID uuid.UUID) (Member, error) {
	m, ok := f.members[userID]
	if !ok {
		return Member{}, ErrNotInAlliance
	}
	return m, nil
}

func (f *fakeStore) ListMembers(_ context.Context, allianceID uuid.UUID) ([]Member, error) {
	var out []Member
	for _, m := range f.members {
		if m.AllianceID == allianceID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeStore) CountMembers(_ context.Context, allianceID uuid.UUID) (int, error) {
	n := 0
	for _, m := range f.members {
		if m.AllianceID == allianceID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) UpdateMemberRole(_ context.Context, userID uuid.UUID, role string) error {
	m, ok := f.members[userID]
	if !ok {
		return ErrNotInAlliance
	}
	m.Role = role
	f.members[userID] = m
	return nil
}

func (f *fakeStore) DeleteMember(_ context.Context, userID uuid.UUID) error {
	delete(f.members, userID)
	return nil
}

func (f *fakeStore) InsertInvite(_ context.Context, inv Invite) (Invite, error) {
	for _, existing := range f.invites {
		if existing.AllianceID == inv.AllianceID &&
			existing.UserID == inv.UserID &&
			existing.Status == "pending" {
			return Invite{}, ErrAlreadyInvited
		}
	}
	f.invites[inv.ID] = inv
	return inv, nil
}

func (f *fakeStore) GetInvite(_ context.Context, id uuid.UUID) (Invite, error) {
	inv, ok := f.invites[id]
	if !ok {
		return Invite{}, ErrInviteNotFound
	}
	return inv, nil
}

func (f *fakeStore) UpdateInviteStatus(_ context.Context, id uuid.UUID, status string) error {
	inv, ok := f.invites[id]
	if !ok {
		return ErrInviteNotFound
	}
	inv.Status = status
	f.invites[id] = inv
	return nil
}

// fakeChatSvc records calls without touching any real state.
type fakeChatSvc struct {
	channels    map[uuid.UUID]string  // channelID → kind
	memberships map[uuid.UUID]map[uuid.UUID]bool // channelID → set of userIDs
	deleted     []uuid.UUID
}

func newFakeChatSvc() *fakeChatSvc {
	return &fakeChatSvc{
		channels:    make(map[uuid.UUID]string),
		memberships: make(map[uuid.UUID]map[uuid.UUID]bool),
	}
}

func (f *fakeChatSvc) CreateChannel(_ context.Context, kind string, _ *uuid.UUID) (chat.Channel, error) {
	ch := chat.Channel{ID: uuid.New(), Kind: kind}
	f.channels[ch.ID] = kind
	return ch, nil
}

func (f *fakeChatSvc) DeleteChannel(_ context.Context, channelID uuid.UUID) error {
	delete(f.channels, channelID)
	f.deleted = append(f.deleted, channelID)
	return nil
}

func (f *fakeChatSvc) EnsureMembership(_ context.Context, channelID, userID uuid.UUID) error {
	if _, ok := f.memberships[channelID]; !ok {
		f.memberships[channelID] = make(map[uuid.UUID]bool)
	}
	f.memberships[channelID][userID] = true
	return nil
}

func (f *fakeChatSvc) RemoveMembership(_ context.Context, channelID, userID uuid.UUID) error {
	if m, ok := f.memberships[channelID]; ok {
		delete(m, userID)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func fixedNow() func() time.Time {
	t := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func newSvc(store *fakeStore, chat *fakeChatSvc) *Service {
	return NewServiceWithStore(store, chat, fixedNow())
}

// setupAlliance creates an alliance with one leader member and returns the IDs.
func setupAlliance(t *testing.T, store *fakeStore, chat *fakeChatSvc) (allianceID, leaderID uuid.UUID) {
	t.Helper()
	leaderID = uuid.New()
	svc := newSvc(store, chat)
	a, err := svc.Create(context.Background(), leaderID, "TestAlliance", "TEST", "")
	if err != nil {
		t.Fatalf("setup: create alliance: %v", err)
	}
	return a.ID, leaderID
}

// ── Create tests ──────────────────────────────────────────────────────────────

func TestCreate_HappyPath(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	svc := newSvc(store, chatSvc)

	leaderID := uuid.New()
	a, err := svc.Create(context.Background(), leaderID, "Warriors", "WAR", "Elite group")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if a.Name != "Warriors" {
		t.Errorf("name = %q, want %q", a.Name, "Warriors")
	}
	if a.LeaderID != leaderID {
		t.Errorf("leader_id = %v, want %v", a.LeaderID, leaderID)
	}
	// Leader must be in alliance_members as "leader".
	m, ok := store.members[leaderID]
	if !ok {
		t.Fatal("leader not in alliance_members")
	}
	if m.Role != "leader" {
		t.Errorf("role = %q, want %q", m.Role, "leader")
	}
	// Chat channel must have been created and leader added.
	if a.ChannelID == nil {
		t.Fatal("channel_id is nil")
	}
	if !chatSvc.memberships[*a.ChannelID][leaderID] {
		t.Error("leader not added to alliance chat channel")
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		allianceName string
		tag         string
		description string
		wantFields  []string
	}{
		{"name too short", "X", "TAG", "", []string{"name"}},
		{"name too long", "AAAAAAAAAAAAAAAAAAAAAAAAA", "TAG", "", []string{"name"}},
		{"tag too short", "Valid", "T", "", []string{"tag"}},
		{"tag too long", "Valid", "TOOLONG", "", []string{"tag"}},
		{"desc too long", "Valid", "TAG", string(make([]byte, 201)), []string{"description"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newSvc(newFakeStore(), newFakeChatSvc())
			_, err := svc.Create(context.Background(), uuid.New(), tt.allianceName, tt.tag, tt.description)
			var ve *models.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			for _, wantField := range tt.wantFields {
				found := false
				for _, fe := range ve.Fields {
					if fe.Field == wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ValidationError missing field %q", wantField)
				}
			}
		})
	}
}

func TestCreate_RejectsAlreadyInAlliance(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	_, leaderID := setupAlliance(t, store, chatSvc)

	svc := newSvc(store, chatSvc)
	_, err := svc.Create(context.Background(), leaderID, "Second", "SC", "")
	if !errors.Is(err, ErrAlreadyInAlliance) {
		t.Errorf("err = %v, want ErrAlreadyInAlliance", err)
	}
}

func TestCreate_NameAndTagUniqueness(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	_, _ = setupAlliance(t, store, chatSvc) // "TestAlliance" / "TEST"

	svc := newSvc(store, chatSvc)

	// Same name
	_, err := svc.Create(context.Background(), uuid.New(), "TestAlliance", "OTH", "")
	if !errors.Is(err, ErrNameTaken) {
		t.Errorf("name conflict: err = %v, want ErrNameTaken", err)
	}

	// Same tag
	_, err = svc.Create(context.Background(), uuid.New(), "Other", "TEST", "")
	if !errors.Is(err, ErrTagTaken) {
		t.Errorf("tag conflict: err = %v, want ErrTagTaken", err)
	}
}

// ── Disband tests ─────────────────────────────────────────────────────────────

func TestDisband_LeaderCanDisband(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	svc := newSvc(store, chatSvc)
	if err := svc.Disband(context.Background(), leaderID, allianceID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	// Alliance and members should be gone.
	if _, ok := store.alliances[allianceID]; ok {
		t.Error("alliance still exists after disband")
	}
	if _, ok := store.members[leaderID]; ok {
		t.Error("leader still in members after disband")
	}
}

func TestDisband_OfficerCannotDisband(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	err := svc.Disband(context.Background(), officerID, allianceID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
	_ = leaderID
}

func TestDisband_MemberCannotDisband(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	err := svc.Disband(context.Background(), memberID, allianceID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestDisband_NotMemberReturnsNotMember(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	// A different alliance's leader.
	otherLeaderID := uuid.New()
	otherAllianceID := uuid.New()
	store.members[otherLeaderID] = Member{UserID: otherLeaderID, AllianceID: otherAllianceID, Role: "leader"}

	svc := newSvc(store, chatSvc)
	err := svc.Disband(context.Background(), otherLeaderID, allianceID)
	if !errors.Is(err, ErrNotMember) {
		t.Errorf("err = %v, want ErrNotMember", err)
	}
}

// ── Invite tests ──────────────────────────────────────────────────────────────

func TestInvite_LeaderCanInvite(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	inv, err := svc.Invite(context.Background(), leaderID, allianceID, targetID)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if inv.UserID != targetID {
		t.Errorf("user_id = %v, want %v", inv.UserID, targetID)
	}
	if inv.Status != "pending" {
		t.Errorf("status = %q, want pending", inv.Status)
	}
}

func TestInvite_OfficerCanInvite(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	_, err := svc.Invite(context.Background(), officerID, allianceID, targetID)
	if err != nil {
		t.Errorf("officer invite: err = %v, want nil", err)
	}
}

func TestInvite_MemberCannotInvite(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	_, err := svc.Invite(context.Background(), memberID, allianceID, uuid.New())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestInvite_CannotInviteExistingAllianceMember(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	_, err := svc.Invite(context.Background(), leaderID, allianceID, memberID)
	if !errors.Is(err, ErrAlreadyInAlliance) {
		t.Errorf("err = %v, want ErrAlreadyInAlliance", err)
	}
}

func TestInvite_DuplicatePendingInvite(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)

	if _, err := svc.Invite(context.Background(), leaderID, allianceID, targetID); err != nil {
		t.Fatalf("first invite: err = %v", err)
	}
	_, err := svc.Invite(context.Background(), leaderID, allianceID, targetID)
	if !errors.Is(err, ErrAlreadyInvited) {
		t.Errorf("duplicate invite: err = %v, want ErrAlreadyInvited", err)
	}
}

// ── AcceptInvite tests ────────────────────────────────────────────────────────

func TestAcceptInvite_HappyPath(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)

	inv, err := svc.Invite(context.Background(), leaderID, allianceID, targetID)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}

	if err := svc.AcceptInvite(context.Background(), targetID, inv.ID); err != nil {
		t.Fatalf("accept: err = %v, want nil", err)
	}

	// Target must now be a member.
	m, ok := store.members[targetID]
	if !ok {
		t.Fatal("target not in alliance_members after accept")
	}
	if m.Role != "member" {
		t.Errorf("role = %q, want member", m.Role)
	}

	// Invite status must be "accepted".
	if store.invites[inv.ID].Status != "accepted" {
		t.Errorf("invite status = %q, want accepted", store.invites[inv.ID].Status)
	}
}

func TestAcceptInvite_WrongUser(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	inv, _ := svc.Invite(context.Background(), leaderID, allianceID, targetID)

	err := svc.AcceptInvite(context.Background(), uuid.New(), inv.ID)
	if !errors.Is(err, ErrInviteNotFound) {
		t.Errorf("err = %v, want ErrInviteNotFound", err)
	}
}

func TestAcceptInvite_AlreadyAccepted(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	inv, _ := svc.Invite(context.Background(), leaderID, allianceID, targetID)
	_ = svc.AcceptInvite(context.Background(), targetID, inv.ID)

	// Second accept must fail.
	err := svc.AcceptInvite(context.Background(), targetID, inv.ID)
	if !errors.Is(err, ErrInviteNotPending) {
		t.Errorf("err = %v, want ErrInviteNotPending", err)
	}
}

func TestAcceptInvite_AlreadyInAlliance(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	// targetID already in an alliance
	targetID := uuid.New()
	store.members[targetID] = Member{UserID: targetID, AllianceID: allianceID, Role: "member"}

	inviteID := uuid.New()
	store.invites[inviteID] = Invite{ID: inviteID, AllianceID: allianceID, UserID: targetID, Status: "pending"}

	svc := newSvc(store, chatSvc)
	err := svc.AcceptInvite(context.Background(), targetID, inviteID)
	if !errors.Is(err, ErrAlreadyInAlliance) {
		t.Errorf("err = %v, want ErrAlreadyInAlliance", err)
	}
	_ = leaderID
}

// ── Leave tests ───────────────────────────────────────────────────────────────

func TestLeave_MemberCanLeave(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	if err := svc.Leave(context.Background(), memberID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := store.members[memberID]; ok {
		t.Error("member still in alliance_members after leave")
	}
}

func TestLeave_LeaderWithOtherMembersCannotLeave(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	err := svc.Leave(context.Background(), leaderID)
	if !errors.Is(err, ErrLeaderMustTransfer) {
		t.Errorf("err = %v, want ErrLeaderMustTransfer", err)
	}
}

func TestLeave_LeaderAloneDisbandsAlliance(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	svc := newSvc(store, chatSvc)
	if err := svc.Leave(context.Background(), leaderID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := store.alliances[allianceID]; ok {
		t.Error("alliance still exists — lone leader leaving should disband")
	}
}

func TestLeave_NotInAlliance(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	svc := newSvc(store, chatSvc)

	err := svc.Leave(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotInAlliance) {
		t.Errorf("err = %v, want ErrNotInAlliance", err)
	}
}

// ── Promote tests ─────────────────────────────────────────────────────────────

func TestPromote_MemberToOfficer(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	if err := svc.Promote(context.Background(), leaderID, allianceID, memberID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if store.members[memberID].Role != "officer" {
		t.Errorf("role = %q, want officer", store.members[memberID].Role)
	}
}

func TestPromote_OfficerToLeader_TransfersLeadership(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	if err := svc.Promote(context.Background(), leaderID, allianceID, officerID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	// Officer is now leader.
	if store.members[officerID].Role != "leader" {
		t.Errorf("new leader role = %q, want leader", store.members[officerID].Role)
	}
	// Old leader is now officer.
	if store.members[leaderID].Role != "officer" {
		t.Errorf("old leader role = %q, want officer", store.members[leaderID].Role)
	}
	// alliances.leader_id updated.
	if store.alliances[allianceID].LeaderID != officerID {
		t.Errorf("alliance.leader_id = %v, want %v", store.alliances[allianceID].LeaderID, officerID)
	}
}

func TestPromote_OfficerCannotPromote(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}
	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	err := svc.Promote(context.Background(), officerID, allianceID, memberID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestPromote_CannotTargetSelf(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	svc := newSvc(store, chatSvc)
	err := svc.Promote(context.Background(), leaderID, allianceID, leaderID)
	if !errors.Is(err, ErrCannotTargetSelf) {
		t.Errorf("err = %v, want ErrCannotTargetSelf", err)
	}
}

// ── Demote tests ──────────────────────────────────────────────────────────────

func TestDemote_LeaderCanDemoteOfficer(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	if err := svc.Demote(context.Background(), leaderID, allianceID, officerID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if store.members[officerID].Role != "member" {
		t.Errorf("role = %q, want member", store.members[officerID].Role)
	}
}

func TestDemote_CannotDemoteMember(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	err := svc.Demote(context.Background(), leaderID, allianceID, memberID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestDemote_OfficerCannotDemote(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}
	officer2ID := uuid.New()
	store.members[officer2ID] = Member{UserID: officer2ID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	err := svc.Demote(context.Background(), officerID, allianceID, officer2ID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

// ── Kick tests ────────────────────────────────────────────────────────────────

func TestKick_LeaderCanKickOfficer(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	if err := svc.Kick(context.Background(), leaderID, allianceID, officerID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if _, ok := store.members[officerID]; ok {
		t.Error("officer still in members after kick")
	}
}

func TestKick_LeaderCanKickMember(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	if err := svc.Kick(context.Background(), leaderID, allianceID, memberID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestKick_OfficerCanKickMember(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}
	memberID := uuid.New()
	store.members[memberID] = Member{UserID: memberID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	if err := svc.Kick(context.Background(), officerID, allianceID, memberID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestKick_OfficerCannotKickOfficer(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officer1ID := uuid.New()
	store.members[officer1ID] = Member{UserID: officer1ID, AllianceID: allianceID, Role: "officer"}
	officer2ID := uuid.New()
	store.members[officer2ID] = Member{UserID: officer2ID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	err := svc.Kick(context.Background(), officer1ID, allianceID, officer2ID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestKick_CannotKickLeader(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	err := svc.Kick(context.Background(), officerID, allianceID, leaderID)
	if !errors.Is(err, ErrCannotKickLeader) {
		t.Errorf("err = %v, want ErrCannotKickLeader", err)
	}
}

func TestKick_MemberCannotKick(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	member1ID := uuid.New()
	store.members[member1ID] = Member{UserID: member1ID, AllianceID: allianceID, Role: "member"}
	member2ID := uuid.New()
	store.members[member2ID] = Member{UserID: member2ID, AllianceID: allianceID, Role: "member"}

	svc := newSvc(store, chatSvc)
	err := svc.Kick(context.Background(), member1ID, allianceID, member2ID)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", err)
	}
}

func TestKick_CannotKickSelf(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, _ := setupAlliance(t, store, chatSvc)

	officerID := uuid.New()
	store.members[officerID] = Member{UserID: officerID, AllianceID: allianceID, Role: "officer"}

	svc := newSvc(store, chatSvc)
	err := svc.Kick(context.Background(), officerID, allianceID, officerID)
	if !errors.Is(err, ErrCannotTargetSelf) {
		t.Errorf("err = %v, want ErrCannotTargetSelf", err)
	}
}

// ── DeclineInvite tests ───────────────────────────────────────────────────────

func TestDeclineInvite_HappyPath(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	inv, _ := svc.Invite(context.Background(), leaderID, allianceID, targetID)

	if err := svc.DeclineInvite(context.Background(), targetID, inv.ID); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if store.invites[inv.ID].Status != "rejected" {
		t.Errorf("status = %q, want rejected", store.invites[inv.ID].Status)
	}
}

func TestDeclineInvite_WrongUser(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	chatSvc := newFakeChatSvc()
	allianceID, leaderID := setupAlliance(t, store, chatSvc)

	targetID := uuid.New()
	svc := newSvc(store, chatSvc)
	inv, _ := svc.Invite(context.Background(), leaderID, allianceID, targetID)

	err := svc.DeclineInvite(context.Background(), uuid.New(), inv.ID)
	if !errors.Is(err, ErrInviteNotFound) {
		t.Errorf("err = %v, want ErrInviteNotFound", err)
	}
}
