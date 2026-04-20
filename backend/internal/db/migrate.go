package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" driver with database/sql
)

//go:embed migrations
var migrationsFS embed.FS

// MigrateUp applies all pending up migrations in lexicographic order.
// It creates the schema_migrations tracking table if it does not yet exist.
func MigrateUp(ctx context.Context, databaseURL string) error {
	db, err := openSQL(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	files, err := listMigrations("up")
	if err != nil {
		return err
	}

	for _, name := range files {
		version := versionOf(name)
		applied, err := isApplied(ctx, db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyFile(ctx, db, name, version); err != nil {
			return err
		}
	}
	return nil
}

// MigrateDown rolls back the most recently applied migration.
// It is a no-op when no migrations have been applied.
func MigrateDown(ctx context.Context, databaseURL string) error {
	db, err := openSQL(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	var version string
	err = db.QueryRowContext(ctx,
		`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`,
	).Scan(&version)
	if err == sql.ErrNoRows {
		return nil // nothing to roll back
	}
	if err != nil {
		return fmt.Errorf("query last migration: %w", err)
	}

	name := "migrations/" + version + ".down.sql"
	content, err := migrationsFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }() // noop after Commit; safe to ignore

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version = $1`, version,
	); err != nil {
		return fmt.Errorf("remove migration record %s: %w", version, err)
	}
	return tx.Commit()
}

// openSQL opens a *sql.DB via the pgx stdlib driver. The caller is responsible
// for closing it.
func openSQL(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db for migration: %w", err)
	}
	return db, nil
}

// ensureSchema creates the schema_migrations table if it does not exist.
func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text        PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

// listMigrations returns sorted paths of all migration files matching direction
// ("up" or "down") from the embedded FS.
func listMigrations(direction string) ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	suffix := "." + direction + ".sql"
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			names = append(names, "migrations/"+e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// versionOf extracts the migration version identifier from a full path.
// e.g. "migrations/0001_init.up.sql" → "0001_init"
func versionOf(path string) string {
	base := path[len("migrations/"):]
	base = strings.TrimSuffix(base, ".up.sql")
	base = strings.TrimSuffix(base, ".down.sql")
	return base
}

// isApplied reports whether the given migration version has been recorded in
// schema_migrations.
func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, version,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check applied %s: %w", version, err)
	}
	return count > 0, nil
}

// applyFile executes a migration SQL file inside a transaction and records the
// version in schema_migrations on success.
func applyFile(ctx context.Context, db *sql.DB, name, version string) error {
	content, err := migrationsFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }() // noop after Commit; safe to ignore

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
	); err != nil {
		return fmt.Errorf("record %s: %w", version, err)
	}
	return tx.Commit()
}
