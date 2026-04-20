package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used when hashing passwords. Override in
// TestMain to bcrypt.MinCost to keep tests fast without changing production
// behaviour.
var bcryptCost = 12

// maxPasswordBytes is the maximum length accepted by bcrypt. Inputs longer
// than 72 bytes are silently truncated by bcrypt; we reject them explicitly.
const maxPasswordBytes = 72

// HashPassword hashes plain using bcrypt. Returns an error if plain exceeds
// maxPasswordBytes or if the OS entropy source fails.
func HashPassword(plain string) (string, error) {
	if len(plain) > maxPasswordBytes {
		return "", fmt.Errorf("password exceeds %d bytes", maxPasswordBytes)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
// Returns ErrInvalidCredentials on mismatch; wraps other errors.
func VerifyPassword(hash, plain string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	return nil
}
