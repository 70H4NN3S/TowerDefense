//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/db"
)

// TestMigrateUpDown_Integration runs the full migration cycle against a real
// PostgreSQL instance. Set DATABASE_URL to enable.
//
// Run with: go test -tags=integration ./internal/db/...
func TestMigrateUpDown_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()

	// Apply all migrations.
	if err := db.MigrateUp(ctx, dbURL); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}

	// Second call must be idempotent.
	if err := db.MigrateUp(ctx, dbURL); err != nil {
		t.Fatalf("MigrateUp (idempotent): %v", err)
	}

	// Roll back the last migration.
	if err := db.MigrateDown(ctx, dbURL); err != nil {
		t.Fatalf("MigrateDown: %v", err)
	}

	// Re-applying after rollback must succeed.
	if err := db.MigrateUp(ctx, dbURL); err != nil {
		t.Fatalf("MigrateUp (after rollback): %v", err)
	}
}
