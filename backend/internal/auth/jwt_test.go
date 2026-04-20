package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("super-secret-test-key-32-bytes!!")

const testSubject = "550e8400-e29b-41d4-a716-446655440000"

func testNow() time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
}

func TestSignAt_ProducesThreeParts(t *testing.T) {
	t.Parallel()

	tok, err := signAt(testSubject, testSecret, time.Hour, testNow())
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Errorf("token has %d parts, want 3", len(parts))
	}
}

func TestParseAt_HappyPath(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	c, err := parseAt(tok, testSecret, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("parseAt: %v", err)
	}
	if c.Sub != testSubject {
		t.Errorf("sub = %q, want %q", c.Sub, testSubject)
	}
	if c.Iss != jwtIssuer {
		t.Errorf("iss = %q, want %q", c.Iss, jwtIssuer)
	}
	if c.Jti == "" {
		t.Error("jti must not be empty")
	}
}

func TestParseAt_Expired(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	// Parse two hours after issue.
	_, err = parseAt(tok, testSecret, now.Add(2*time.Hour))
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("err = %v, want ErrExpiredToken", err)
	}
}

func TestParseAt_ExactlyAtExpiry(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	// now.Unix() == exp means expired (>= check).
	_, err = parseAt(tok, testSecret, now.Add(time.Hour))
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("err = %v, want ErrExpiredToken at exact expiry boundary", err)
	}
}

func TestParseAt_TamperedSignature(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	tampered := tok[:len(tok)-4] + "XXXX"
	_, err = parseAt(tampered, testSecret, now.Add(time.Minute))
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken for tampered signature", err)
	}
}

func TestParseAt_WrongSecret(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	_, err = parseAt(tok, []byte("wrong-secret-key-32-bytes-here!!"), now.Add(time.Minute))
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken for wrong secret", err)
	}
}

func TestParseAt_TamperedPayload(t *testing.T) {
	t.Parallel()

	now := testNow()
	tok, err := signAt(testSubject, testSecret, time.Hour, now)
	if err != nil {
		t.Fatalf("signAt: %v", err)
	}

	parts := strings.Split(tok, ".")
	// Replace payload with a different base64 blob.
	parts[1] = parts[1] + "x"
	tampered := strings.Join(parts, ".")

	_, err = parseAt(tampered, testSecret, now.Add(time.Minute))
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken for tampered payload", err)
	}
}

func TestParseAt_MalformedToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"one part", "onlyone"},
		{"two parts", "a.b"},
		{"four parts", "a.b.c.d"},
		{"garbage", "not.a.jwt"},
	}

	now := testNow()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseAt(tt.token, testSecret, now)
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestSignAt_UniqJTI(t *testing.T) {
	t.Parallel()

	now := testNow()
	seen := make(map[string]struct{}, 100)
	for range 100 {
		tok, err := signAt(testSubject, testSecret, time.Hour, now)
		if err != nil {
			t.Fatalf("signAt: %v", err)
		}
		c, err := parseAt(tok, testSecret, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("parseAt: %v", err)
		}
		if _, dup := seen[c.Jti]; dup {
			t.Fatalf("duplicate JTI after %d iterations", len(seen))
		}
		seen[c.Jti] = struct{}{}
	}
}
