// Package chat implements the chat service: channels, memberships, and messages.
package chat

import "errors"

// Sentinel errors returned by the chat package. Callers match with errors.Is.
var (
	// ErrChannelNotFound is returned when no chat_channels row matches.
	ErrChannelNotFound = errors.New("channel not found")

	// ErrNotMember is returned when a user tries to access a channel they
	// have not joined.
	ErrNotMember = errors.New("not a member of this channel")

	// ErrBodyEmpty is returned when Send receives an empty message body.
	ErrBodyEmpty = errors.New("message body is empty")

	// ErrBodyTooLong is returned when Send receives a body over 500 characters.
	ErrBodyTooLong = errors.New("message body exceeds 500 characters")
)
