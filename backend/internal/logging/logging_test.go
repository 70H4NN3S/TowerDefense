package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestNewWithWriter_Level(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configLevel string
		msgLevel    slog.Level
		wantLogged  bool
	}{
		{"debug config logs debug", "debug", slog.LevelDebug, true},
		{"info config skips debug", "info", slog.LevelDebug, false},
		{"info config logs info", "info", slog.LevelInfo, true},
		{"warn config skips info", "warn", slog.LevelInfo, false},
		{"warn config logs warn", "warn", slog.LevelWarn, true},
		{"error config skips warn", "error", slog.LevelWarn, false},
		{"error config logs error", "error", slog.LevelError, true},
		{"unknown level defaults to info", "invalid", slog.LevelInfo, true},
		{"unknown level skips debug", "invalid", slog.LevelDebug, false},
		{"warning alias for warn", "warning", slog.LevelWarn, true},
		{"case insensitive DEBUG", "DEBUG", slog.LevelDebug, true},
		{"case insensitive INFO", "INFO", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			log := newWithWriter(tt.configLevel, &buf)
			log.Log(context.Background(), tt.msgLevel, "test message")

			logged := buf.Len() > 0
			if logged != tt.wantLogged {
				t.Errorf("logged = %v, want %v (configLevel=%q, msgLevel=%v)",
					logged, tt.wantLogged, tt.configLevel, tt.msgLevel)
			}
		})
	}
}

func TestNew_ReturnsLogger(t *testing.T) {
	t.Parallel()
	log := New("info")
	if log == nil {
		t.Fatal("New returned nil")
	}
}
