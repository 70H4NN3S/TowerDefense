package events

import "errors"

// Sentinel errors returned by Engine methods. Callers match with errors.Is.
var (
	// ErrEventNotFound is returned when no event matches the given ID.
	ErrEventNotFound = errors.New("event not found")
	// ErrEventNotActive is returned when a claim is attempted outside the event window.
	ErrEventNotActive = errors.New("event is not active")
	// ErrTierInvalid is returned when the requested tier index is out of bounds.
	ErrTierInvalid = errors.New("tier index out of range")
	// ErrTierNotReached is returned when the user's progress has not met the tier threshold.
	ErrTierNotReached = errors.New("tier threshold not reached")
	// ErrTierAlreadyClaimed is returned when the user already claimed the requested tier.
	ErrTierAlreadyClaimed = errors.New("tier already claimed")
)
