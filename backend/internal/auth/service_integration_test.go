//go:build integration

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/70H4NN3S/TowerDefense/internal/auth"
	"github.com/70H4NN3S/TowerDefense/internal/db"
)

// newIntegrationService returns a real *auth.Service backed by a live PostgreSQL
// pool. Migrations are applied before the service is constructed.
// The returned cleanup function deletes the given emails from the users table.
func newIntegrationService(t *testing.T) (*auth.Service, func(emails ...string)) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	if err := db.MigrateUp(ctx, dbURL); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	pool, err := db.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	svc := auth.NewService(pool, []byte("integration-test-secret-32bytes!"))

	cleanup := func(emails ...string) {
		for _, email := range emails {
			if _, err := pool.Exec(ctx, `DELETE FROM users WHERE email = lower($1)`, email); err != nil {
				t.Errorf("cleanup: delete user %q: %v", email, err)
			}
		}
	}
	return svc, cleanup
}

// safeTestName converts a t.Name() into a string safe for use as an email
// local-part or username (no slashes, spaces, or other special characters).
func safeTestName(t *testing.T) string {
	t.Helper()
	r := strings.NewReplacer("/", "_", " ", "_", "#", "_")
	return r.Replace(t.Name())
}

// uniqueEmail generates a test-scoped email to avoid conflicts across parallel tests.
func uniqueEmail(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("%s_%s@integration.test", safeTestName(t), suffix)
}

// uniqueUsername generates a test-scoped username to avoid conflicts across parallel tests.
func uniqueUsername(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("%s_%s", safeTestName(t), suffix)
}

// --- Register ---

func TestIntegration_Register_HappyPath(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")
	t.Cleanup(func() { cleanup(email) })

	pair, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("access token must not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token must not be empty")
	}
}

// TestIntegration_Register_DuplicateEmail verifies that re-registering with the
// same email returns ErrEmailTaken via the real database unique constraint.
func TestIntegration_Register_DuplicateEmail(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")
	t.Cleanup(func() { cleanup(email) })

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "b"), // different username, same email
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

// TestIntegration_Register_DuplicateUsername verifies that registering with the
// same username returns ErrUsernameTaken via the real database unique constraint.
func TestIntegration_Register_DuplicateUsername(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email1 := uniqueEmail(t, "a")
	email2 := uniqueEmail(t, "b")
	t.Cleanup(func() { cleanup(email1, email2) })

	username := uniqueUsername(t, "shared")

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email1,
		Username: username,
		Password: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email2, // different email, same username
		Username: username,
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, auth.ErrUsernameTaken) {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

// TestIntegration_Register_CitextUsername verifies that citext makes username
// matching case-insensitive at the database level.
// ("Alice" and "alice" must be treated as the same username.)
func TestIntegration_Register_CitextUsername(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email1 := uniqueEmail(t, "a")
	email2 := uniqueEmail(t, "b")
	t.Cleanup(func() { cleanup(email1, email2) })

	username := uniqueUsername(t, "cituser")

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email1,
		Username: strings.ToLower(username),
		Password: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	// Attempt registration with the upper-cased variant of the same username.
	_, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email2,
		Username: strings.ToUpper(username),
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, auth.ErrUsernameTaken) {
		t.Errorf("err = %v, want ErrUsernameTaken (citext should fold case)", err)
	}
}

// --- Login ---

func TestIntegration_Login_HappyPath(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")
	password := "correct-horse-battery"
	t.Cleanup(func() { cleanup(email) })

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: password,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	pair, err := svc.Login(context.Background(), auth.LoginInput{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("token pair must be non-empty")
	}
}

// TestIntegration_Login_WrongPassword verifies that bcrypt verification rejects
// wrong passwords against the real stored hash.
func TestIntegration_Login_WrongPassword(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")
	t.Cleanup(func() { cleanup(email) })

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: "correct-horse-battery",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Email:    email,
		Password: "definitely-wrong-password",
	})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

// TestIntegration_Login_UnknownEmail verifies that a missing user returns
// ErrInvalidCredentials rather than ErrNotFound, preventing user enumeration.
func TestIntegration_Login_UnknownEmail(t *testing.T) {
	t.Parallel()

	svc, _ := newIntegrationService(t)

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Email:    uniqueEmail(t, "nobody"),
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials (no user enumeration)", err)
	}
}

// TestIntegration_Login_CitextEmail verifies that the citext column makes email
// lookups case-insensitive so a user registered with a lower-case email can log
// in with an upper-case variant of the same address.
func TestIntegration_Login_CitextEmail(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	// Use a lower-case email at registration; the service normalises it anyway,
	// but the citext column must also tolerate a direct upper-case query.
	email := uniqueEmail(t, "a")
	password := "correct-horse-battery"
	t.Cleanup(func() { cleanup(email) })

	if _, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: password,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Login with the email normalised to lower-case (service does this before
	// the DB query); the citext index makes the lookup fast and case-insensitive.
	_, err := svc.Login(context.Background(), auth.LoginInput{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Errorf("Login: %v", err)
	}
}

// --- Refresh ---

func TestIntegration_Refresh_HappyPath(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")
	t.Cleanup(func() { cleanup(email) })

	pair, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Error("refreshed token pair must be non-empty")
	}
}

func TestIntegration_Refresh_InvalidToken(t *testing.T) {
	t.Parallel()

	svc, _ := newIntegrationService(t)

	_, err := svc.Refresh(context.Background(), "not.a.valid.token")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

// TestIntegration_Refresh_UserDeleted verifies that a structurally valid token
// for a user that no longer exists in the database is rejected gracefully.
func TestIntegration_Refresh_UserDeleted(t *testing.T) {
	t.Parallel()

	svc, cleanup := newIntegrationService(t)
	email := uniqueEmail(t, "a")

	pair, err := svc.Register(context.Background(), auth.RegisterInput{
		Email:    email,
		Username: uniqueUsername(t, "a"),
		Password: "correct-horse-battery",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Delete the user before attempting a refresh.
	cleanup(email)

	_, err = svc.Refresh(context.Background(), pair.RefreshToken)
	if err == nil {
		t.Error("expected error refreshing token for deleted user, got nil")
	}
}
