// Package uuid provides a minimal UUID v4 implementation backed by crypto/rand.
// It avoids an external dependency for a capability that fits in ~50 lines.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
)

// UUID is a canonical lowercase UUID v4 string
// (e.g. "550e8400-e29b-41d4-a716-446655440000").
type UUID string

var reUUID = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

// ErrInvalid is returned by Parse when the input is not a valid v4 UUID.
var ErrInvalid = errors.New("invalid UUID")

// New generates a random (version 4) UUID. It panics if the OS entropy source
// fails, which should never happen on a healthy system.
func New() UUID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("uuid: crypto/rand failed: " + err.Error())
	}
	// Set version 4 bits.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits (RFC 4122).
	b[8] = (b[8] & 0x3f) | 0x80

	enc := hex.EncodeToString(b[:])
	return UUID(enc[0:8] + "-" + enc[8:12] + "-" + enc[12:16] + "-" + enc[16:20] + "-" + enc[20:32])
}

// Parse validates s as a canonical lowercase UUID v4 string and returns it.
// Returns ErrInvalid if the input is malformed or not version 4.
func Parse(s string) (UUID, error) {
	if !reUUID.MatchString(s) {
		return "", ErrInvalid
	}
	return UUID(s), nil
}

// MustParse is like Parse but panics on error. Use in tests and init-time constants.
func MustParse(s string) UUID {
	u, err := Parse(s)
	if err != nil {
		panic("uuid.MustParse: " + err.Error() + ": " + s)
	}
	return u
}

// String returns the UUID as a plain string.
func (u UUID) String() string { return string(u) }
