package uuid

import (
	"strings"
	"testing"
)

func TestNew_FormatIsValid(t *testing.T) {
	t.Parallel()

	u := New()
	if _, err := Parse(u.String()); err != nil {
		t.Errorf("New() produced invalid UUID %q: %v", u, err)
	}
}

func TestNew_IsUnique(t *testing.T) {
	t.Parallel()

	const n = 1000
	seen := make(map[UUID]struct{}, n)
	for i := range n {
		u := New()
		if _, dup := seen[u]; dup {
			t.Fatalf("duplicate UUID %q after %d iterations", u, i)
		}
		seen[u] = struct{}{}
	}
}

func TestNew_Version4(t *testing.T) {
	t.Parallel()

	for range 100 {
		u := string(New())
		// Position 14 must be '4' (version nibble).
		if u[14] != '4' {
			t.Errorf("position 14 = %q, want '4'", u[14])
		}
		// Position 19 must be '8', '9', 'a', or 'b' (variant nibble).
		variant := u[19]
		if variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
			t.Errorf("position 19 = %q, want one of 89ab", variant)
		}
	}
}

func TestNew_IsLowercase(t *testing.T) {
	t.Parallel()

	u := string(New())
	if u != strings.ToLower(u) {
		t.Errorf("New() = %q contains uppercase letters", u)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid v4", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid v4 variant b", "550e8400-e29b-41d4-b716-446655440000", false},
		{"empty string", "", true},
		{"missing hyphens", "550e8400e29b41d4a716446655440000", true},
		{"version 1", "550e8400-e29b-11d4-a716-446655440000", true},
		{"uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"wrong length", "550e8400-e29b-41d4-a716-44665544000", true},
		{"all zeros", "00000000-0000-0000-0000-000000000000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if err == nil && string(u) != tt.input {
				t.Errorf("Parse(%q) = %q, want same value back", tt.input, u)
			}
		})
	}
}

func TestMustParse_Panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse with invalid input did not panic")
		}
	}()
	MustParse("not-a-uuid")
}

func TestMustParse_ValidDoesNotPanic(t *testing.T) {
	t.Parallel()

	u := MustParse("550e8400-e29b-41d4-a716-446655440000")
	if u == "" {
		t.Error("MustParse returned empty UUID")
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	const raw = "550e8400-e29b-41d4-a716-446655440000"
	u := UUID(raw)
	if u.String() != raw {
		t.Errorf("String() = %q, want %q", u.String(), raw)
	}
}
