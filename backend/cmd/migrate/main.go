// Package main is the entry point for the Tower Defense migration runner.
// Usage: migrate <up|down>
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/johannesniedens/towerdefense/internal/config"
	"github.com/johannesniedens/towerdefense/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		slog.Error("usage: migrate <up|down>")
		os.Exit(1)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "up":
		if err := db.MigrateUp(ctx, cfg.DatabaseURL); err != nil {
			slog.Error("migration up failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")
	case "down":
		if err := db.MigrateDown(ctx, cfg.DatabaseURL); err != nil {
			slog.Error("migration down failed", "err", err)
			os.Exit(1)
		}
		slog.Info("migration rolled back")
	default:
		slog.Error("unknown command; expected up or down", "cmd", os.Args[1])
		os.Exit(1)
	}
}
