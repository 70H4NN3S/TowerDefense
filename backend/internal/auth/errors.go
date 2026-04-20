package auth

import "errors"

// Sentinel errors that callers match with errors.Is.
var (
	// ErrNotFound is returned when a requested user does not exist.
	ErrNotFound = errors.New("not found")

	// ErrEmailTaken is returned when the email is already registered.
	ErrEmailTaken = errors.New("email already taken")

	// ErrUsernameTaken is returned when the username is already registered.
	ErrUsernameTaken = errors.New("username already taken")

	// ErrInvalidCredentials is returned on a failed login attempt.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrExpiredToken is returned when a JWT has passed its expiry time.
	ErrExpiredToken = errors.New("token expired")

	// ErrInvalidToken is returned when a JWT is malformed, tampered with, or
	// otherwise cannot be verified.
	ErrInvalidToken = errors.New("invalid token")
)
