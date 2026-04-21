// Package alliance implements the alliance service: creation, membership
// management, invites, and role-based permissions.
package alliance

import "errors"

// Sentinel errors returned by the alliance package. Callers match with errors.Is.
var (
	// ErrNotFound is returned when no alliances row matches the given ID.
	ErrNotFound = errors.New("alliance not found")

	// ErrNameTaken is returned when the requested alliance name is already in use.
	ErrNameTaken = errors.New("alliance name is already taken")

	// ErrTagTaken is returned when the requested alliance tag is already in use.
	ErrTagTaken = errors.New("alliance tag is already taken")

	// ErrAlreadyInAlliance is returned when a user tries to create or join an
	// alliance while already being a member of one.
	ErrAlreadyInAlliance = errors.New("already a member of an alliance")

	// ErrNotInAlliance is returned when an action requires the caller to be in
	// an alliance but they are not.
	ErrNotInAlliance = errors.New("not a member of any alliance")

	// ErrNotMember is returned when the requester is not a member of the
	// specific alliance the action targets.
	ErrNotMember = errors.New("not a member of this alliance")

	// ErrPermissionDenied is returned when the caller's role is insufficient
	// for the requested action (e.g. officer trying to disband).
	ErrPermissionDenied = errors.New("insufficient permissions for this action")

	// ErrLeaderMustTransfer is returned when the alliance leader tries to leave
	// while other members still exist. The leader must promote a new leader or
	// disband the alliance first.
	ErrLeaderMustTransfer = errors.New("leader must transfer leadership or disband before leaving")

	// ErrInviteNotFound is returned when no alliance_invites row matches.
	ErrInviteNotFound = errors.New("invite not found")

	// ErrInviteNotPending is returned when the invite's status is not "pending".
	ErrInviteNotPending = errors.New("invite is no longer pending")

	// ErrAlreadyInvited is returned when the target user already has a pending
	// invite from this alliance.
	ErrAlreadyInvited = errors.New("user already has a pending invite to this alliance")

	// ErrCannotTargetSelf is returned when the requester and the target are the
	// same user (e.g. kicking yourself).
	ErrCannotTargetSelf = errors.New("cannot target yourself")

	// ErrCannotKickLeader is returned when someone tries to kick the alliance
	// leader.
	ErrCannotKickLeader = errors.New("cannot kick the alliance leader")
)
