package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	// Override cost to minimum so tests don't take 300 ms per hash.
	bcryptCost = bcrypt.MinCost
	m.Run()
}

func TestHashPassword_HappyPath(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Error("hash must not be empty")
	}
}

func TestHashPassword_ExceedsMaxBytes(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", maxPasswordBytes+1)
	_, err := HashPassword(long)
	if err == nil {
		t.Error("expected error for password exceeding max bytes, got nil")
	}
}

func TestHashPassword_AtMaxBytes(t *testing.T) {
	t.Parallel()

	exact := strings.Repeat("a", maxPasswordBytes)
	_, err := HashPassword(exact)
	if err != nil {
		t.Errorf("HashPassword at max bytes: %v", err)
	}
}

func TestHashPassword_ProducesUniqueHashes(t *testing.T) {
	t.Parallel()

	h1, err1 := HashPassword("same-password")
	h2, err2 := HashPassword("same-password")
	if err1 != nil || err2 != nil {
		t.Fatalf("HashPassword errors: %v, %v", err1, err2)
	}
	if h1 == h2 {
		t.Error("two calls with the same input must not produce the same hash (bcrypt should salt)")
	}
}

func TestVerifyPassword_Match(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("my-password-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := VerifyPassword(hash, "my-password-123"); err != nil {
		t.Errorf("VerifyPassword: %v", err)
	}
}

func TestVerifyPassword_Mismatch(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	err = VerifyPassword(hash, "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestVerifyPassword_EmptyPlain(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("nonempty")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	err = VerifyPassword(hash, "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}
