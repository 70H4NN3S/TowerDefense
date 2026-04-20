// Package logging provides a thin constructor over log/slog.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON *slog.Logger writing to stderr at the given level string.
// Recognised values are "debug", "info", "warn"/"warning", and "error".
// Any unrecognised value defaults to info.
func New(level string) *slog.Logger {
	return newWithWriter(level, os.Stderr)
}

// newWithWriter is the testable core of New; it writes to w instead of stderr.
func newWithWriter(level string, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewJSONHandler(w, opts))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
