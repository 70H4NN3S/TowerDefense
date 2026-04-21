// Package game implements the core game-state logic: resources, towers,
// matches, and the simulation. It does not contain HTTP or WebSocket code.
package game

import "errors"

// Sentinel errors returned by the game package. Callers match with errors.Is.
var (
	// ErrProfileNotFound is returned when no profile row exists for a user.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrInsufficientGold is returned when a spend would make gold negative.
	ErrInsufficientGold = errors.New("insufficient gold")

	// ErrInsufficientDiamonds is returned when a spend would make diamonds
	// negative.
	ErrInsufficientDiamonds = errors.New("insufficient diamonds")

	// ErrInsufficientEnergy is returned when a spend would make energy
	// negative.
	ErrInsufficientEnergy = errors.New("insufficient energy")

	// ErrTemplateNotFound is returned when no tower_templates row matches.
	ErrTemplateNotFound = errors.New("tower template not found")

	// ErrAlreadyOwned is returned when the player tries to buy a tower they
	// already own.
	ErrAlreadyOwned = errors.New("tower already owned")

	// ErrNotOwned is returned when an action targets a tower the player does
	// not own.
	ErrNotOwned = errors.New("tower not owned")

	// ErrMaxLevel is returned when the player tries to upgrade past level 10.
	ErrMaxLevel = errors.New("tower is already at max level")

	// ErrMatchNotFound is returned when no matches row matches the given ID.
	ErrMatchNotFound = errors.New("match not found")

	// ErrMatchAlreadyEnded is returned when SubmitResult is called on a match
	// that already has an ended_at timestamp.
	ErrMatchAlreadyEnded = errors.New("match already ended")

	// ErrMatchNotOwned is returned when the requesting user is not player_one
	// of the match.
	ErrMatchNotOwned = errors.New("match not owned by this player")

	// ErrUnknownMap is returned when StartSinglePlayer receives a map ID that
	// does not exist in the sim registry.
	ErrUnknownMap = errors.New("unknown map id")

	// ErrAlreadyQueued is returned when a player tries to join matchmaking
	// but is already waiting in the queue.
	ErrAlreadyQueued = errors.New("already in matchmaking queue")

	// ErrNotQueued is returned when a player tries to leave matchmaking
	// but is not currently in the queue.
	ErrNotQueued = errors.New("not in matchmaking queue")
)
