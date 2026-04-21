package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/70H4NN3S/TowerDefense/internal/alliance"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── fakeAllianceSvc ───────────────────────────────────────────────────────────

type fakeAllianceSvc struct {
	alliance   *alliance.Alliance
	member     *alliance.Member
	members    []alliance.Member
	invite     *alliance.Invite
	createErr  error
	getErr     error
	memberErr  error
	listErr    error
	disbandErr error
	inviteErr  error
	acceptErr  error
	declineErr error
	leaveErr   error
	promoteErr error
	demoteErr  error
	kickErr    error
}

func (f *fakeAllianceSvc) Create(_ context.Context, leaderID uuid.UUID, name, tag, description string) (alliance.Alliance, error) {
	if f.createErr != nil {
		return alliance.Alliance{}, f.createErr
	}
	if f.alliance != nil {
		return *f.alliance, nil
	}
	chID := uuid.New()
	return alliance.Alliance{
		ID:          uuid.New(),
		Name:        name,
		Tag:         tag,
		Description: description,
		LeaderID:    leaderID,
		ChannelID:   &chID,
		CreatedAt:   time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeAllianceSvc) Get(_ context.Context, _ uuid.UUID) (alliance.Alliance, error) {
	if f.getErr != nil {
		return alliance.Alliance{}, f.getErr
	}
	if f.alliance != nil {
		return *f.alliance, nil
	}
	return alliance.Alliance{ID: uuid.New(), Name: "Stub", Tag: "STB", CreatedAt: time.Now()}, nil
}

func (f *fakeAllianceSvc) GetMembership(_ context.Context, _ uuid.UUID) (alliance.Member, error) {
	if f.memberErr != nil {
		return alliance.Member{}, f.memberErr
	}
	if f.member != nil {
		return *f.member, nil
	}
	return alliance.Member{UserID: uuid.New(), AllianceID: uuid.New(), Role: "member", JoinedAt: time.Now()}, nil
}

func (f *fakeAllianceSvc) ListMembers(_ context.Context, _ uuid.UUID) ([]alliance.Member, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.members, nil
}

func (f *fakeAllianceSvc) Disband(_ context.Context, _, _ uuid.UUID) error    { return f.disbandErr }
func (f *fakeAllianceSvc) Leave(_ context.Context, _ uuid.UUID) error          { return f.leaveErr }
func (f *fakeAllianceSvc) Promote(_ context.Context, _, _, _ uuid.UUID) error  { return f.promoteErr }
func (f *fakeAllianceSvc) Demote(_ context.Context, _, _, _ uuid.UUID) error   { return f.demoteErr }
func (f *fakeAllianceSvc) Kick(_ context.Context, _, _, _ uuid.UUID) error     { return f.kickErr }

func (f *fakeAllianceSvc) Invite(_ context.Context, _, _, targetUserID uuid.UUID) (alliance.Invite, error) {
	if f.inviteErr != nil {
		return alliance.Invite{}, f.inviteErr
	}
	if f.invite != nil {
		return *f.invite, nil
	}
	return alliance.Invite{
		ID:         uuid.New(),
		AllianceID: uuid.New(),
		UserID:     targetUserID,
		Status:     "pending",
		CreatedAt:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (f *fakeAllianceSvc) AcceptInvite(_ context.Context, _, _ uuid.UUID) error  { return f.acceptErr }
func (f *fakeAllianceSvc) DeclineInvite(_ context.Context, _, _ uuid.UUID) error { return f.declineErr }

// ── helpers ───────────────────────────────────────────────────────────────────

func newAllianceMux(svc *fakeAllianceSvc) *http.ServeMux {
	mux := http.NewServeMux()
	NewAllianceHandler(svc, testSecret).Register(mux)
	return mux
}

func alliancePath(id uuid.UUID) string    { return fmt.Sprintf("/v1/alliances/%s", id) }
func membersPath(id uuid.UUID) string     { return fmt.Sprintf("/v1/alliances/%s/members", id) }
func invitesPath(id uuid.UUID) string     { return fmt.Sprintf("/v1/alliances/%s/invites", id) }
func memberActionPath(aID, uID uuid.UUID, action string) string {
	return fmt.Sprintf("/v1/alliances/%s/members/%s/%s", aID, uID, action)
}
func kickPath(aID, uID uuid.UUID) string { return fmt.Sprintf("/v1/alliances/%s/members/%s", aID, uID) }
func inviteActionPath(id uuid.UUID, action string) string {
	return fmt.Sprintf("/v1/invites/%s/%s", id, action)
}

// ── POST /v1/alliances ────────────────────────────────────────────────────────

func TestAllianceCreate_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	w := doRequest(mux, http.MethodPost, "/v1/alliances", "", `{"name":"X","tag":"XX"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAllianceCreate_MalformedBody_Returns400(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/alliances", tok, `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAllianceCreate_HappyPath_Returns201(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/alliances", tok,
		`{"name":"Warriors","tag":"WAR","description":"Elite"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["alliance"]; !ok {
		t.Error("response missing 'alliance' key")
	}
}

func TestAllianceCreate_ServiceError_Propagates(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{createErr: alliance.ErrAlreadyInAlliance})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/alliances", tok,
		`{"name":"Warriors","tag":"WAR"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	assertErrorCode(t, w.Body.Bytes(), "already_in_alliance")
}

// ── GET /v1/alliances/{id} ────────────────────────────────────────────────────

func TestAllianceGet_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	w := doRequest(mux, http.MethodGet, alliancePath(uuid.New()), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAllianceGet_InvalidUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/alliances/not-a-uuid", tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAllianceGet_NotFound_Returns404(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{getErr: alliance.ErrNotFound})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, alliancePath(uuid.New()), tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestAllianceGet_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, alliancePath(uuid.New()), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["alliance"]; !ok {
		t.Error("response missing 'alliance' key")
	}
}

// ── DELETE /v1/alliances/{id} (disband) ───────────────────────────────────────

func TestAllianceDisband_Unauthenticated_Returns401(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	w := doRequest(mux, http.MethodDelete, alliancePath(uuid.New()), "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestAllianceDisband_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{disbandErr: alliance.ErrPermissionDenied})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodDelete, alliancePath(uuid.New()), tok, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestAllianceDisband_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodDelete, alliancePath(uuid.New()), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

// ── GET /v1/alliances/{id}/members ───────────────────────────────────────────

func TestAllianceListMembers_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	svc := &fakeAllianceSvc{members: []alliance.Member{
		{UserID: uuid.New(), AllianceID: uuid.New(), Role: "leader", JoinedAt: time.Now()},
	}}
	mux := newAllianceMux(svc)
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, membersPath(uuid.New()), tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	members, ok := resp["members"].([]any)
	if !ok {
		t.Fatal("response missing 'members' array")
	}
	if len(members) != 1 {
		t.Errorf("len(members) = %d, want 1", len(members))
	}
}

// ── POST /v1/alliances/{id}/invites ──────────────────────────────────────────

func TestAllianceInvite_HappyPath_Returns201(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, invitesPath(uuid.New()), tok,
		fmt.Sprintf(`{"user_id":"%s"}`, uuid.New()))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["invite"]; !ok {
		t.Error("response missing 'invite' key")
	}
}

func TestAllianceInvite_InvalidUserID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, invitesPath(uuid.New()), tok,
		`{"user_id":"not-a-uuid"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAllianceInvite_AlreadyInvited_Returns409(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{inviteErr: alliance.ErrAlreadyInvited})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, invitesPath(uuid.New()), tok,
		fmt.Sprintf(`{"user_id":"%s"}`, uuid.New()))
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestAllianceInvite_MemberCannotInvite_Returns403(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{inviteErr: alliance.ErrPermissionDenied})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, invitesPath(uuid.New()), tok,
		fmt.Sprintf(`{"user_id":"%s"}`, uuid.New()))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// ── POST /v1/invites/{id}/accept ─────────────────────────────────────────────

func TestAllianceAcceptInvite_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, inviteActionPath(uuid.New(), "accept"), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestAllianceAcceptInvite_NotPending_Returns409(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{acceptErr: alliance.ErrInviteNotPending})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, inviteActionPath(uuid.New(), "accept"), tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestAllianceAcceptInvite_AlreadyInAlliance_Returns409(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{acceptErr: alliance.ErrAlreadyInAlliance})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, inviteActionPath(uuid.New(), "accept"), tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

// ── POST /v1/invites/{id}/decline ────────────────────────────────────────────

func TestAllianceDeclineInvite_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, inviteActionPath(uuid.New(), "decline"), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestAllianceDeclineInvite_NotFound_Returns404(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{declineErr: alliance.ErrInviteNotFound})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, inviteActionPath(uuid.New(), "decline"), tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// ── POST /v1/me/alliance/leave ───────────────────────────────────────────────

func TestAllianceLeave_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/me/alliance/leave", tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestAllianceLeave_LeaderMustTransfer_Returns409(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{leaveErr: alliance.ErrLeaderMustTransfer})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/me/alliance/leave", tok, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
}

func TestAllianceLeave_NotInAlliance_Returns404(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{leaveErr: alliance.ErrNotInAlliance})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, "/v1/me/alliance/leave", tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// ── GET /v1/me/alliance ──────────────────────────────────────────────────────

func TestAllianceGetMembership_HappyPath_Returns200(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/me/alliance", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := resp["membership"]; !ok {
		t.Error("response missing 'membership' key")
	}
}

func TestAllianceGetMembership_NotInAlliance_Returns404(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{memberErr: alliance.ErrNotInAlliance})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodGet, "/v1/me/alliance", tok, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// ── POST /v1/alliances/{id}/members/{userID}/promote ─────────────────────────

func TestAlliancePromote_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, memberActionPath(uuid.New(), uuid.New(), "promote"), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestAlliancePromote_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{promoteErr: alliance.ErrPermissionDenied})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, memberActionPath(uuid.New(), uuid.New(), "promote"), tok, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// ── POST /v1/alliances/{id}/members/{userID}/demote ──────────────────────────

func TestAllianceDemote_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodPost, memberActionPath(uuid.New(), uuid.New(), "demote"), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

// ── DELETE /v1/alliances/{id}/members/{userID} (kick) ────────────────────────

func TestAllianceKick_HappyPath_Returns204(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodDelete, kickPath(uuid.New(), uuid.New()), tok, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestAllianceKick_CannotKickLeader_Returns403(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{kickErr: alliance.ErrCannotKickLeader})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodDelete, kickPath(uuid.New(), uuid.New()), tok, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestAllianceKick_InvalidTargetUUID_Returns400(t *testing.T) {
	t.Parallel()
	mux := newAllianceMux(&fakeAllianceSvc{})
	tok := signedToken(t, uuid.New())
	w := doRequest(mux, http.MethodDelete,
		fmt.Sprintf("/v1/alliances/%s/members/not-a-uuid", uuid.New()), tok, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ── shared assertion helpers ──────────────────────────────────────────────────

func assertErrorCode(t *testing.T, body []byte, wantCode string) {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal error envelope: %v", err)
	}
	if env.Error.Code != wantCode {
		t.Errorf("error.code = %q, want %q", env.Error.Code, wantCode)
	}
}
