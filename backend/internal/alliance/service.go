package alliance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/70H4NN3S/TowerDefense/internal/chat"
	"github.com/70H4NN3S/TowerDefense/internal/models"
	"github.com/70H4NN3S/TowerDefense/internal/uuid"
)

// ── domain types ──────────────────────────────────────────────────────────────

// Alliance is a persisted alliance row.
type Alliance struct {
	ID          uuid.UUID
	Name        string
	Tag         string
	Description string
	LeaderID    uuid.UUID
	ChannelID   *uuid.UUID // nil if the chat channel was deleted
	CreatedAt   time.Time
}

// Member is an alliance_members row.
type Member struct {
	UserID     uuid.UUID
	AllianceID uuid.UUID
	Role       string // "leader" | "officer" | "member"
	JoinedAt   time.Time
}

// Invite is an alliance_invites row.
type Invite struct {
	ID         uuid.UUID
	AllianceID uuid.UUID
	UserID     uuid.UUID
	Status     string // "pending" | "accepted" | "rejected"
	CreatedAt  time.Time
}

// ── store interface ───────────────────────────────────────────────────────────

// Store abstracts alliance persistence. Declared consumer-side so tests can use fakes.
type Store interface {
	// InsertAlliance persists a new alliance row.
	InsertAlliance(ctx context.Context, a Alliance) (Alliance, error)
	// GetAlliance returns the alliance with the given ID.
	GetAlliance(ctx context.Context, id uuid.UUID) (Alliance, error)
	// SetAllianceLeader updates leader_id on the alliances row.
	SetAllianceLeader(ctx context.Context, allianceID, newLeaderID uuid.UUID) error
	// DeleteAlliance removes the alliance and cascades to members and invites.
	DeleteAlliance(ctx context.Context, id uuid.UUID) error

	// InsertMember adds a user to an alliance with the given role.
	InsertMember(ctx context.Context, m Member) (Member, error)
	// GetMember returns the alliance membership for a user. A user can only be
	// in one alliance at a time (user_id is the PK of alliance_members).
	GetMember(ctx context.Context, userID uuid.UUID) (Member, error)
	// ListMembers returns all members of an alliance ordered by joined_at ASC.
	ListMembers(ctx context.Context, allianceID uuid.UUID) ([]Member, error)
	// CountMembers returns the number of members in an alliance.
	CountMembers(ctx context.Context, allianceID uuid.UUID) (int, error)
	// UpdateMemberRole changes the role of an existing member.
	UpdateMemberRole(ctx context.Context, userID uuid.UUID, role string) error
	// DeleteMember removes a user from their alliance.
	DeleteMember(ctx context.Context, userID uuid.UUID) error

	// InsertInvite persists a new pending invite.
	InsertInvite(ctx context.Context, inv Invite) (Invite, error)
	// GetInvite returns the invite with the given ID.
	GetInvite(ctx context.Context, id uuid.UUID) (Invite, error)
	// UpdateInviteStatus sets the status of an invite to "accepted" or "rejected".
	UpdateInviteStatus(ctx context.Context, id uuid.UUID, status string) error
}

// ── chat interface ────────────────────────────────────────────────────────────

// ChatService is the subset of chat.Service used by the alliance service.
type ChatService interface {
	CreateChannel(ctx context.Context, kind string, ownerID *uuid.UUID) (chat.Channel, error)
	DeleteChannel(ctx context.Context, channelID uuid.UUID) error
	EnsureMembership(ctx context.Context, channelID, userID uuid.UUID) error
	RemoveMembership(ctx context.Context, channelID, userID uuid.UUID) error
}

// ── validation constants ──────────────────────────────────────────────────────

const (
	maxNameLen = 24
	minNameLen = 2
	maxTagLen  = 6
	minTagLen  = 2
	maxDescLen = 200
)

// ── service ───────────────────────────────────────────────────────────────────

// Service implements the alliance domain: creation, membership, invites, and
// role-based permission enforcement.
type Service struct {
	store   Store
	chatSvc ChatService
	now     func() time.Time
}

// NewService constructs a Service backed by pool and chatSvc.
func NewService(pool *pgxpool.Pool, chatSvc ChatService) *Service {
	return &Service{
		store:   newPGStore(pool),
		chatSvc: chatSvc,
		now:     time.Now,
	}
}

// NewServiceWithStore constructs a Service from explicit dependencies.
// Intended for tests.
func NewServiceWithStore(store Store, chatSvc ChatService, now func() time.Time) *Service {
	return &Service{store: store, chatSvc: chatSvc, now: now}
}

// Create creates a new alliance with leaderID as the founding leader.
// Returns ErrAlreadyInAlliance if leaderID is already in an alliance.
// Name and tag uniqueness violations surface as ErrNameTaken / ErrTagTaken.
func (s *Service) Create(ctx context.Context, leaderID uuid.UUID, name, tag, description string) (Alliance, error) {
	name = strings.TrimSpace(name)
	tag = strings.TrimSpace(tag)
	description = strings.TrimSpace(description)
	if err := validateCreateInputs(name, tag, description); err != nil {
		return Alliance{}, err
	}

	// Fail fast if the user is already in an alliance.
	if _, err := s.store.GetMember(ctx, leaderID); err == nil {
		return Alliance{}, ErrAlreadyInAlliance
	} else if !errors.Is(err, ErrNotInAlliance) {
		return Alliance{}, fmt.Errorf("create: check membership: %w", err)
	}

	// Create the alliance chat channel first so we can store its ID.
	ch, err := s.chatSvc.CreateChannel(ctx, "alliance", nil)
	if err != nil {
		return Alliance{}, fmt.Errorf("create: chat channel: %w", err)
	}

	chID := ch.ID
	a := Alliance{
		ID:          uuid.New(),
		Name:        name,
		Tag:         tag,
		Description: description,
		LeaderID:    leaderID,
		ChannelID:   &chID,
		CreatedAt:   s.now(),
	}
	a, err = s.store.InsertAlliance(ctx, a)
	if err != nil {
		// Best-effort cleanup of the chat channel; ignore secondary error.
		_ = s.chatSvc.DeleteChannel(ctx, ch.ID)
		return Alliance{}, fmt.Errorf("create: insert alliance: %w", err)
	}

	// Add the leader as a member.
	_, err = s.store.InsertMember(ctx, Member{
		UserID:     leaderID,
		AllianceID: a.ID,
		Role:       "leader",
		JoinedAt:   s.now(),
	})
	if err != nil {
		return Alliance{}, fmt.Errorf("create: insert leader member: %w", err)
	}

	// Add leader to the alliance chat channel.
	if err := s.chatSvc.EnsureMembership(ctx, ch.ID, leaderID); err != nil {
		return Alliance{}, fmt.Errorf("create: chat membership: %w", err)
	}

	return a, nil
}

// Get returns the alliance with the given ID.
func (s *Service) Get(ctx context.Context, allianceID uuid.UUID) (Alliance, error) {
	a, err := s.store.GetAlliance(ctx, allianceID)
	if err != nil {
		return Alliance{}, fmt.Errorf("get alliance: %w", err)
	}
	return a, nil
}

// GetMembership returns the alliance membership for userID.
// Returns ErrNotInAlliance if the user is not in any alliance.
func (s *Service) GetMembership(ctx context.Context, userID uuid.UUID) (Member, error) {
	m, err := s.store.GetMember(ctx, userID)
	if err != nil {
		return Member{}, fmt.Errorf("get membership: %w", err)
	}
	return m, nil
}

// ListMembers returns all members of allianceID ordered by joined_at.
func (s *Service) ListMembers(ctx context.Context, allianceID uuid.UUID) ([]Member, error) {
	members, err := s.store.ListMembers(ctx, allianceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	return members, nil
}

// Disband deletes the alliance and its chat channel. Only the alliance leader
// may disband. Returns ErrPermissionDenied for any other role.
func (s *Service) Disband(ctx context.Context, requesterID, allianceID uuid.UUID) error {
	m, err := s.store.GetMember(ctx, requesterID)
	if err != nil {
		return fmt.Errorf("disband: check membership: %w", err)
	}
	if m.AllianceID != allianceID {
		return ErrNotMember
	}
	if m.Role != "leader" {
		return ErrPermissionDenied
	}

	a, err := s.store.GetAlliance(ctx, allianceID)
	if err != nil {
		return fmt.Errorf("disband: get alliance: %w", err)
	}

	// Delete the alliance (cascades to alliance_members and alliance_invites).
	if err := s.store.DeleteAlliance(ctx, allianceID); err != nil {
		return fmt.Errorf("disband: delete alliance: %w", err)
	}

	// Delete the associated chat channel if it still exists.
	// Non-fatal: the alliance row is already removed. Log so orphaned channels
	// can be found and cleaned up if the error recurs.
	if a.ChannelID != nil {
		if err := s.chatSvc.DeleteChannel(ctx, *a.ChannelID); err != nil {
			slog.Warn("alliance: disband could not delete chat channel",
				"err", err,
				"alliance_id", allianceID,
				"channel_id", a.ChannelID,
			)
		}
	}

	return nil
}

// Invite creates a pending invite for targetUserID to join allianceID.
// Only leaders and officers may invite. Returns ErrAlreadyInvited if a
// pending invite already exists.
func (s *Service) Invite(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) (Invite, error) {
	m, err := s.store.GetMember(ctx, requesterID)
	if err != nil {
		return Invite{}, fmt.Errorf("invite: check requester: %w", err)
	}
	if m.AllianceID != allianceID {
		return Invite{}, ErrNotMember
	}
	if m.Role == "member" {
		return Invite{}, ErrPermissionDenied
	}

	// Target must not already be in an alliance.
	if _, err := s.store.GetMember(ctx, targetUserID); err == nil {
		return Invite{}, ErrAlreadyInAlliance
	} else if !errors.Is(err, ErrNotInAlliance) {
		return Invite{}, fmt.Errorf("invite: check target membership: %w", err)
	}

	inv, err := s.store.InsertInvite(ctx, Invite{
		ID:         uuid.New(),
		AllianceID: allianceID,
		UserID:     targetUserID,
		Status:     "pending",
		CreatedAt:  s.now(),
	})
	if err != nil {
		return Invite{}, fmt.Errorf("invite: insert: %w", err)
	}
	return inv, nil
}

// AcceptInvite accepts a pending invite on behalf of userID.
// Returns ErrInviteNotFound, ErrInviteNotPending, or ErrAlreadyInAlliance.
func (s *Service) AcceptInvite(ctx context.Context, userID, inviteID uuid.UUID) error {
	inv, err := s.store.GetInvite(ctx, inviteID)
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	if inv.UserID != userID {
		return ErrInviteNotFound // Don't reveal that the invite exists.
	}
	if inv.Status != "pending" {
		return ErrInviteNotPending
	}

	// User must not already be in an alliance.
	if _, err := s.store.GetMember(ctx, userID); err == nil {
		return ErrAlreadyInAlliance
	} else if !errors.Is(err, ErrNotInAlliance) {
		return fmt.Errorf("accept invite: check membership: %w", err)
	}

	_, err = s.store.InsertMember(ctx, Member{
		UserID:     userID,
		AllianceID: inv.AllianceID,
		Role:       "member",
		JoinedAt:   s.now(),
	})
	if err != nil {
		return fmt.Errorf("accept invite: insert member: %w", err)
	}

	if err := s.store.UpdateInviteStatus(ctx, inviteID, "accepted"); err != nil {
		return fmt.Errorf("accept invite: update status: %w", err)
	}

	// Add to alliance chat channel if it still exists.
	a, err := s.store.GetAlliance(ctx, inv.AllianceID)
	if err == nil && a.ChannelID != nil {
		_ = s.chatSvc.EnsureMembership(ctx, *a.ChannelID, userID)
	}

	return nil
}

// DeclineInvite rejects a pending invite on behalf of userID.
func (s *Service) DeclineInvite(ctx context.Context, userID, inviteID uuid.UUID) error {
	inv, err := s.store.GetInvite(ctx, inviteID)
	if err != nil {
		return fmt.Errorf("decline invite: %w", err)
	}
	if inv.UserID != userID {
		return ErrInviteNotFound
	}
	if inv.Status != "pending" {
		return ErrInviteNotPending
	}
	if err := s.store.UpdateInviteStatus(ctx, inviteID, "rejected"); err != nil {
		return fmt.Errorf("decline invite: update status: %w", err)
	}
	return nil
}

// Leave removes userID from their current alliance.
// Returns ErrLeaderMustTransfer if the user is the leader and other members exist.
// If the leader is the only member, the alliance is disbanded automatically.
func (s *Service) Leave(ctx context.Context, userID uuid.UUID) error {
	m, err := s.store.GetMember(ctx, userID)
	if err != nil {
		return fmt.Errorf("leave: %w", err)
	}

	if m.Role == "leader" {
		count, err := s.store.CountMembers(ctx, m.AllianceID)
		if err != nil {
			return fmt.Errorf("leave: count members: %w", err)
		}
		if count > 1 {
			return ErrLeaderMustTransfer
		}
		// Sole member — disband by deleting the alliance.
		return s.Disband(ctx, userID, m.AllianceID)
	}

	a, err := s.store.GetAlliance(ctx, m.AllianceID)
	if err != nil {
		return fmt.Errorf("leave: get alliance: %w", err)
	}

	if err := s.store.DeleteMember(ctx, userID); err != nil {
		return fmt.Errorf("leave: delete member: %w", err)
	}

	if a.ChannelID != nil {
		_ = s.chatSvc.RemoveMembership(ctx, *a.ChannelID, userID)
	}

	return nil
}

// Promote advances targetUserID's role within allianceID.
//   - member  → officer  (leader only)
//   - officer → leader   (leader only; transfers leadership — old leader becomes officer)
//
// Returns ErrPermissionDenied if the requester is not the leader.
func (s *Service) Promote(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error {
	if requesterID == targetUserID {
		return ErrCannotTargetSelf
	}

	requester, err := s.store.GetMember(ctx, requesterID)
	if err != nil {
		return fmt.Errorf("promote: check requester: %w", err)
	}
	if requester.AllianceID != allianceID || requester.Role != "leader" {
		return ErrPermissionDenied
	}

	target, err := s.store.GetMember(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("promote: check target: %w", err)
	}
	if target.AllianceID != allianceID {
		return ErrNotMember
	}

	switch target.Role {
	case "member":
		// member → officer
		if err := s.store.UpdateMemberRole(ctx, targetUserID, "officer"); err != nil {
			return fmt.Errorf("promote: update target role: %w", err)
		}
	case "officer":
		// officer → leader (leadership transfer)
		if err := s.store.UpdateMemberRole(ctx, targetUserID, "leader"); err != nil {
			return fmt.Errorf("promote: update target to leader: %w", err)
		}
		if err := s.store.UpdateMemberRole(ctx, requesterID, "officer"); err != nil {
			return fmt.Errorf("promote: demote old leader: %w", err)
		}
		if err := s.store.SetAllianceLeader(ctx, allianceID, targetUserID); err != nil {
			return fmt.Errorf("promote: set alliance leader: %w", err)
		}
	case "leader":
		// Target is already the leader — nothing to do.
		return ErrPermissionDenied
	}

	return nil
}

// Demote reduces targetUserID's role from officer to member within allianceID.
// Only the leader may demote. Returns ErrPermissionDenied otherwise.
func (s *Service) Demote(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error {
	if requesterID == targetUserID {
		return ErrCannotTargetSelf
	}

	requester, err := s.store.GetMember(ctx, requesterID)
	if err != nil {
		return fmt.Errorf("demote: check requester: %w", err)
	}
	if requester.AllianceID != allianceID || requester.Role != "leader" {
		return ErrPermissionDenied
	}

	target, err := s.store.GetMember(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("demote: check target: %w", err)
	}
	if target.AllianceID != allianceID {
		return ErrNotMember
	}
	if target.Role != "officer" {
		return ErrPermissionDenied
	}

	if err := s.store.UpdateMemberRole(ctx, targetUserID, "member"); err != nil {
		return fmt.Errorf("demote: update role: %w", err)
	}
	return nil
}

// Kick removes targetUserID from allianceID.
//   - Leader may kick officers and members.
//   - Officer may kick members only.
//   - Nobody may kick the leader.
//   - Nobody may kick themselves (use Leave instead).
func (s *Service) Kick(ctx context.Context, requesterID, allianceID, targetUserID uuid.UUID) error {
	if requesterID == targetUserID {
		return ErrCannotTargetSelf
	}

	requester, err := s.store.GetMember(ctx, requesterID)
	if err != nil {
		return fmt.Errorf("kick: check requester: %w", err)
	}
	if requester.AllianceID != allianceID {
		return ErrNotMember
	}
	if requester.Role == "member" {
		return ErrPermissionDenied
	}

	target, err := s.store.GetMember(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("kick: check target: %w", err)
	}
	if target.AllianceID != allianceID {
		return ErrNotMember
	}
	if target.Role == "leader" {
		return ErrCannotKickLeader
	}
	// Officer can only kick members.
	if requester.Role == "officer" && target.Role == "officer" {
		return ErrPermissionDenied
	}

	a, err := s.store.GetAlliance(ctx, allianceID)
	if err != nil {
		return fmt.Errorf("kick: get alliance: %w", err)
	}

	if err := s.store.DeleteMember(ctx, targetUserID); err != nil {
		return fmt.Errorf("kick: delete member: %w", err)
	}

	if a.ChannelID != nil {
		_ = s.chatSvc.RemoveMembership(ctx, *a.ChannelID, targetUserID)
	}

	return nil
}

// ── validation helpers ────────────────────────────────────────────────────────

func validateCreateInputs(name, tag, description string) error {
	var ve models.ValidationError
	nameLen := utf8.RuneCountInString(name)
	tagLen := utf8.RuneCountInString(tag)
	descLen := utf8.RuneCountInString(description)

	if nameLen < minNameLen || nameLen > maxNameLen {
		ve.Add("name", fmt.Sprintf("must be %d–%d characters", minNameLen, maxNameLen))
	}
	if tagLen < minTagLen || tagLen > maxTagLen {
		ve.Add("tag", fmt.Sprintf("must be %d–%d characters", minTagLen, maxTagLen))
	}
	if descLen > maxDescLen {
		ve.Add("description", fmt.Sprintf("must be at most %d characters", maxDescLen))
	}
	if ve.HasErrors() {
		return &ve
	}
	return nil
}

// ── DB store implementation ───────────────────────────────────────────────────

type pgStore struct {
	pool *pgxpool.Pool
}

func newPGStore(pool *pgxpool.Pool) *pgStore {
	return &pgStore{pool: pool}
}

func (s *pgStore) InsertAlliance(ctx context.Context, a Alliance) (Alliance, error) {
	const q = `
		INSERT INTO alliances (id, name, tag, description, leader_id, channel_id, created_at)
		VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6, $7)
		RETURNING id::text, name, tag, description, leader_id::text, channel_id::text, created_at`

	var idStr, leaderStr string
	var chanStr *string
	err := s.pool.QueryRow(ctx, q,
		a.ID.String(), a.Name, a.Tag, a.Description,
		a.LeaderID.String(), uuidPtrToStr(a.ChannelID), a.CreatedAt,
	).Scan(&idStr, &a.Name, &a.Tag, &a.Description, &leaderStr, &chanStr, &a.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, "uq_alliances_name") {
			return Alliance{}, ErrNameTaken
		}
		if isUniqueViolation(err, "uq_alliances_tag") {
			return Alliance{}, ErrTagTaken
		}
		return Alliance{}, fmt.Errorf("insert alliance: %w", err)
	}
	return parseAllianceRow(idStr, leaderStr, chanStr, a)
}

func (s *pgStore) GetAlliance(ctx context.Context, id uuid.UUID) (Alliance, error) {
	const q = `
		SELECT id::text, name, tag, description, leader_id::text, channel_id::text, created_at
		FROM   alliances
		WHERE  id = $1::uuid`

	var a Alliance
	var idStr, leaderStr string
	var chanStr *string
	err := s.pool.QueryRow(ctx, q, id.String()).Scan(
		&idStr, &a.Name, &a.Tag, &a.Description, &leaderStr, &chanStr, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Alliance{}, ErrNotFound
	}
	if err != nil {
		return Alliance{}, fmt.Errorf("get alliance: %w", err)
	}
	return parseAllianceRow(idStr, leaderStr, chanStr, a)
}

func (s *pgStore) SetAllianceLeader(ctx context.Context, allianceID, newLeaderID uuid.UUID) error {
	const q = `UPDATE alliances SET leader_id = $1::uuid WHERE id = $2::uuid`
	_, err := s.pool.Exec(ctx, q, newLeaderID.String(), allianceID.String())
	if err != nil {
		return fmt.Errorf("set alliance leader: %w", err)
	}
	return nil
}

func (s *pgStore) DeleteAlliance(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM alliances WHERE id = $1::uuid`
	_, err := s.pool.Exec(ctx, q, id.String())
	if err != nil {
		return fmt.Errorf("delete alliance: %w", err)
	}
	return nil
}

func (s *pgStore) InsertMember(ctx context.Context, m Member) (Member, error) {
	const q = `
		INSERT INTO alliance_members (user_id, alliance_id, role, joined_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING user_id::text, alliance_id::text, role, joined_at`

	var uStr, aStr string
	err := s.pool.QueryRow(ctx, q,
		m.UserID.String(), m.AllianceID.String(), m.Role, m.JoinedAt,
	).Scan(&uStr, &aStr, &m.Role, &m.JoinedAt)
	if err != nil {
		return Member{}, fmt.Errorf("insert member: %w", err)
	}
	return parseMemberRow(uStr, aStr, m)
}

func (s *pgStore) GetMember(ctx context.Context, userID uuid.UUID) (Member, error) {
	const q = `
		SELECT user_id::text, alliance_id::text, role, joined_at
		FROM   alliance_members
		WHERE  user_id = $1::uuid`

	var m Member
	var uStr, aStr string
	err := s.pool.QueryRow(ctx, q, userID.String()).Scan(&uStr, &aStr, &m.Role, &m.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNotInAlliance
	}
	if err != nil {
		return Member{}, fmt.Errorf("get member: %w", err)
	}
	return parseMemberRow(uStr, aStr, m)
}

func (s *pgStore) ListMembers(ctx context.Context, allianceID uuid.UUID) ([]Member, error) {
	const q = `
		SELECT user_id::text, alliance_id::text, role, joined_at
		FROM   alliance_members
		WHERE  alliance_id = $1::uuid
		ORDER BY joined_at ASC`

	rows, err := s.pool.Query(ctx, q, allianceID.String())
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		var uStr, aStr string
		if err := rows.Scan(&uStr, &aStr, &m.Role, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		parsed, err := parseMemberRow(uStr, aStr, m)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, rows.Err()
}

func (s *pgStore) CountMembers(ctx context.Context, allianceID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM alliance_members WHERE alliance_id = $1::uuid`
	var n int
	if err := s.pool.QueryRow(ctx, q, allianceID.String()).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

func (s *pgStore) UpdateMemberRole(ctx context.Context, userID uuid.UUID, role string) error {
	const q = `UPDATE alliance_members SET role = $1 WHERE user_id = $2::uuid`
	_, err := s.pool.Exec(ctx, q, role, userID.String())
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

func (s *pgStore) DeleteMember(ctx context.Context, userID uuid.UUID) error {
	const q = `DELETE FROM alliance_members WHERE user_id = $1::uuid`
	_, err := s.pool.Exec(ctx, q, userID.String())
	if err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	return nil
}

func (s *pgStore) InsertInvite(ctx context.Context, inv Invite) (Invite, error) {
	const q = `
		INSERT INTO alliance_invites (id, alliance_id, user_id, status, created_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
		RETURNING id::text, alliance_id::text, user_id::text, status, created_at`

	var idStr, aStr, uStr string
	err := s.pool.QueryRow(ctx, q,
		inv.ID.String(), inv.AllianceID.String(), inv.UserID.String(), inv.Status, inv.CreatedAt,
	).Scan(&idStr, &aStr, &uStr, &inv.Status, &inv.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, "uq_alliance_invites_pending") {
			return Invite{}, ErrAlreadyInvited
		}
		return Invite{}, fmt.Errorf("insert invite: %w", err)
	}
	return parseInviteRow(idStr, aStr, uStr, inv)
}

func (s *pgStore) GetInvite(ctx context.Context, id uuid.UUID) (Invite, error) {
	const q = `
		SELECT id::text, alliance_id::text, user_id::text, status, created_at
		FROM   alliance_invites
		WHERE  id = $1::uuid`

	var inv Invite
	var idStr, aStr, uStr string
	err := s.pool.QueryRow(ctx, q, id.String()).Scan(&idStr, &aStr, &uStr, &inv.Status, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, ErrInviteNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("get invite: %w", err)
	}
	return parseInviteRow(idStr, aStr, uStr, inv)
}

func (s *pgStore) UpdateInviteStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE alliance_invites SET status = $1 WHERE id = $2::uuid`
	_, err := s.pool.Exec(ctx, q, status, id.String())
	if err != nil {
		return fmt.Errorf("update invite status: %w", err)
	}
	return nil
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func parseAllianceRow(idStr, leaderStr string, chanStr *string, a Alliance) (Alliance, error) {
	var err error
	a.ID, err = uuid.Parse(idStr)
	if err != nil {
		return Alliance{}, fmt.Errorf("parse alliance id %q: %w", idStr, err)
	}
	a.LeaderID, err = uuid.Parse(leaderStr)
	if err != nil {
		return Alliance{}, fmt.Errorf("parse leader id %q: %w", leaderStr, err)
	}
	if chanStr != nil {
		cid, err := uuid.Parse(*chanStr)
		if err != nil {
			return Alliance{}, fmt.Errorf("parse channel id %q: %w", *chanStr, err)
		}
		a.ChannelID = &cid
	}
	return a, nil
}

func parseMemberRow(uStr, aStr string, m Member) (Member, error) {
	var err error
	m.UserID, err = uuid.Parse(uStr)
	if err != nil {
		return Member{}, fmt.Errorf("parse member user_id %q: %w", uStr, err)
	}
	m.AllianceID, err = uuid.Parse(aStr)
	if err != nil {
		return Member{}, fmt.Errorf("parse member alliance_id %q: %w", aStr, err)
	}
	return m, nil
}

func parseInviteRow(idStr, aStr, uStr string, inv Invite) (Invite, error) {
	var err error
	inv.ID, err = uuid.Parse(idStr)
	if err != nil {
		return Invite{}, fmt.Errorf("parse invite id %q: %w", idStr, err)
	}
	inv.AllianceID, err = uuid.Parse(aStr)
	if err != nil {
		return Invite{}, fmt.Errorf("parse invite alliance_id %q: %w", aStr, err)
	}
	inv.UserID, err = uuid.Parse(uStr)
	if err != nil {
		return Invite{}, fmt.Errorf("parse invite user_id %q: %w", uStr, err)
	}
	return inv, nil
}

// uuidPtrToStr converts a *uuid.UUID to *string for nullable UUID parameters.
func uuidPtrToStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation (23505)
// for the named constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == constraint
}
