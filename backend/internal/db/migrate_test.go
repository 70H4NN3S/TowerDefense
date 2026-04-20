package db

import (
	"testing"
)

func TestVersionOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"up migration", "migrations/0001_init.up.sql", "0001_init"},
		{"down migration", "migrations/0001_init.down.sql", "0001_init"},
		{"multi-digit number", "migrations/0042_users.up.sql", "0042_users"},
		{"hyphenated description", "migrations/0003_alliance-members.up.sql", "0003_alliance-members"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := versionOf(tt.path); got != tt.want {
				t.Errorf("versionOf(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestListMigrations_ReturnsSortedUpFiles(t *testing.T) {
	t.Parallel()

	files, err := listMigrations("up")
	if err != nil {
		t.Fatalf("listMigrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one up migration file")
	}
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("migrations not sorted: %q before %q", files[i-1], files[i])
		}
	}
}

func TestListMigrations_ReturnsSortedDownFiles(t *testing.T) {
	t.Parallel()

	files, err := listMigrations("down")
	if err != nil {
		t.Fatalf("listMigrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one down migration file")
	}
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("down migrations not sorted: %q before %q", files[i-1], files[i])
		}
	}
}

func TestListMigrations_UpDownParity(t *testing.T) {
	t.Parallel()

	ups, err := listMigrations("up")
	if err != nil {
		t.Fatalf("listMigrations up: %v", err)
	}
	downs, err := listMigrations("down")
	if err != nil {
		t.Fatalf("listMigrations down: %v", err)
	}
	if len(ups) != len(downs) {
		t.Errorf("up migrations (%d) != down migrations (%d); every up needs a down", len(ups), len(downs))
	}
	for i := range ups {
		if i >= len(downs) {
			break
		}
		if versionOf(ups[i]) != versionOf(downs[i]) {
			t.Errorf("version mismatch at index %d: up=%q down=%q",
				i, versionOf(ups[i]), versionOf(downs[i]))
		}
	}
}
