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
)
