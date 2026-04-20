package config

import (
	"strings"
	"testing"
)

// newEnv builds a getenv func from a key/value map for use with loadFrom.
func newEnv(kv map[string]string) func(string) string {
	return func(key string) string { return kv[key] }
}

func TestLoadFrom(t *testing.T) {
	t.Parallel()

	validBase := map[string]string{
		"DATABASE_URL": "postgres://localhost/test",
		"JWT_SECRET":   "a-secret-that-is-at-least-32-characters",
	}

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string // substring; empty means no error expected
		check   func(*testing.T, *Config)
	}{
		{
			name:    "all required fields set",
			env:     validBase,
			wantErr: "",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.ListenAddr != ":8080" {
					t.Errorf("ListenAddr = %q, want default %q", c.ListenAddr, ":8080")
				}
				if c.LogLevel != "info" {
					t.Errorf("LogLevel = %q, want default %q", c.LogLevel, "info")
				}
			},
		},
		{
			name:    "missing DATABASE_URL",
			env:     map[string]string{"JWT_SECRET": "a-secret-that-is-at-least-32-characters"},
			wantErr: "DATABASE_URL",
		},
		{
			name:    "missing JWT_SECRET",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/test"},
			wantErr: "JWT_SECRET",
		},
		{
			name: "JWT_SECRET too short",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"JWT_SECRET":   "tooshort",
			},
			wantErr: "32 characters",
		},
		{
			name:    "both required fields missing",
			env:     map[string]string{},
			wantErr: "DATABASE_URL",
		},
		{
			name: "custom listen addr and log level",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"JWT_SECRET":   "a-secret-that-is-at-least-32-characters",
				"LISTEN_ADDR":  ":9090",
				"LOG_LEVEL":    "debug",
			},
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.ListenAddr != ":9090" {
					t.Errorf("ListenAddr = %q, want %q", c.ListenAddr, ":9090")
				}
				if c.LogLevel != "debug" {
					t.Errorf("LogLevel = %q, want %q", c.LogLevel, "debug")
				}
			},
		},
		{
			name: "JWT_SECRET exactly 32 characters",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"JWT_SECRET":   "12345678901234567890123456789012",
			},
			wantErr: "",
		},
		{
			name: "JWT_SECRET 31 characters",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/test",
				"JWT_SECRET":   "1234567890123456789012345678901",
			},
			wantErr: "32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := loadFrom(newEnv(tt.env))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
