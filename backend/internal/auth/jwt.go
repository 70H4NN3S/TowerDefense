package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const jwtIssuer = "towerdefense"

// jwtHeaderType has deterministic field order so the base64 encoding is
// reproducible. map[string]string has non-deterministic iteration order in Go.
type jwtHeaderType struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

var encodedHeader = func() string {
	b, err := json.Marshal(jwtHeaderType{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		panic("jwt: failed to marshal fixed header: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}()

// Claims is the JWT payload. All fields are required on parse.
type Claims struct {
	Sub string `json:"sub"` // user ID (UUID string)
	Iss string `json:"iss"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Jti string `json:"jti"` // token ID (for revocation)
}

// SignToken signs a JWT for subject (a UUID string) with the given secret and
// TTL. It uses the real clock. Prefer signAt in tests.
func SignToken(subject string, secret []byte, ttl time.Duration) (string, error) {
	return signAt(subject, secret, ttl, time.Now())
}

// signAt is the testable core of SignToken. now is the clock reference.
func signAt(subject string, secret []byte, ttl time.Duration, now time.Time) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	claims := Claims{
		Sub: subject,
		Iss: jwtIssuer,
		Iat: now.Unix(),
		Exp: now.Add(ttl).Unix(),
		Jti: jti,
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	sig := sign(secret, encodedHeader+"."+encodedPayload)
	return encodedHeader + "." + encodedPayload + "." + sig, nil
}

// ParseToken verifies the token signature and expiry, returning the Claims.
// Uses the real clock. Prefer parseAt in tests.
func ParseToken(token string, secret []byte) (Claims, error) {
	return parseAt(token, secret, time.Now())
}

// parseAt is the testable core of ParseToken. now is the clock reference.
func parseAt(token string, secret []byte, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	// Verify header is the one we produce.
	if parts[0] != encodedHeader {
		return Claims{}, ErrInvalidToken
	}

	// Verify signature.
	expectedSig := sign(secret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return Claims{}, ErrInvalidToken
	}

	// Decode payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}

	var c Claims
	dec := json.NewDecoder(strings.NewReader(string(payloadBytes)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Claims{}, ErrInvalidToken
	}

	// Validate required claims.
	if c.Sub == "" || c.Iss != jwtIssuer || c.Iat == 0 || c.Exp == 0 || c.Jti == "" {
		return Claims{}, ErrInvalidToken
	}

	// Check expiry.
	if now.Unix() >= c.Exp {
		return Claims{}, ErrExpiredToken
	}

	return c, nil
}

// sign produces the HMAC-SHA256 signature for the given data.
func sign(secret []byte, data string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// newJTI generates a random 16-byte token ID encoded as hex.
func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use base64 to keep the JTI compact.
	return base64.RawURLEncoding.EncodeToString(b), nil
}
