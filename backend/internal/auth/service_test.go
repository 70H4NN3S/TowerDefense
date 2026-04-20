package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/johannesniedens/towerdefense/internal/models"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// fakeStore is an in-memory implementation of userStoreIface for unit tests.
type fakeStore struct {
	byEmail map[string]User
	byID    map[uuid.UUID]User
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		byEmail: make(map[string]User),
		byID:    make(map[uuid.UUID]User),
	}
}

func (f *fakeStore) CreateUser(_ context.Context, nu NewUser) (User, error) {
	if _, exists := f.byEmail[nu.Email]; exists {
		return User{}, ErrEmailTaken
	}
	for _, u := range f.byEmail {
		if u.Username == nu.Username {
			return User{}, ErrUsernameTaken
		}
	}
	id := uuid.New()
	u := User{ID: id, Email: nu.Email, Username: nu.Username, PasswordHash: nu.PasswordHash}
	f.byEmail[nu.Email] = u
	f.byID[id] = u
	return u, nil
}

func (f *fakeStore) GetUserByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) GetUserByID(_ context.Context, id uuid.UUID) (User, error) {
	u, ok := f.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

// newTestService returns a Service wired to a fakeStore.
func newTestService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		store:     newFakeStore(),
		jwtSecret: []byte("test-secret-32-bytes-for-testing!"),
	}
}

var validRegisterInput = RegisterInput{
	Email:    "alice@example.com",
	Username: "alice",
	Password: "correct-horse-battery",
}

// --- Register ---

func TestRegister_HappyPath(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	pair, err := svc.Register(context.Background(), validRegisterInput)
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

func TestRegister_DuplicateEmail(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	if _, err := svc.Register(context.Background(), validRegisterInput); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    validRegisterInput.Email,
		Username: "bob",
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	if _, err := svc.Register(context.Background(), validRegisterInput); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "bob@example.com",
		Username: validRegisterInput.Username,
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestRegister_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RegisterInput
		field string
	}{
		{
			"empty email",
			RegisterInput{Email: "", Username: "alice", Password: "correct-horse-battery"},
			"email",
		},
		{
			"invalid email",
			RegisterInput{Email: "notanemail", Username: "alice", Password: "correct-horse-battery"},
			"email",
		},
		{
			"empty username",
			RegisterInput{Email: "a@b.com", Username: "", Password: "correct-horse-battery"},
			"username",
		},
		{
			"username too long",
			RegisterInput{Email: "a@b.com", Username: strings.Repeat("a", maxUsernameLen+1), Password: "correct-horse-battery"},
			"username",
		},
		{
			"password too short",
			RegisterInput{Email: "a@b.com", Username: "alice", Password: "short"},
			"password",
		},
		{
			"empty password",
			RegisterInput{Email: "a@b.com", Username: "alice", Password: ""},
			"password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newTestService(t)
			_, err := svc.Register(context.Background(), tt.input)

			var ve *models.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			found := false
			for _, fe := range ve.Fields {
				if fe.Field == tt.field {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("field %q not in validation errors: %+v", tt.field, ve.Fields)
			}
		})
	}
}

// --- Login ---

func TestLogin_HappyPath(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	if _, err := svc.Register(context.Background(), validRegisterInput); err != nil {
		t.Fatalf("Register: %v", err)
	}

	pair, err := svc.Login(context.Background(), LoginInput{
		Email:    validRegisterInput.Email,
		Password: validRegisterInput.Password,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("token pair must be non-empty")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	if _, err := svc.Register(context.Background(), validRegisterInput); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    validRegisterInput.Email,
		Password: "wrong-password!!",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "nobody@example.com",
		Password: "correct-horse-battery",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials (no user enumeration)", err)
	}
}

func TestLogin_EmailCaseInsensitive(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	if _, err := svc.Register(context.Background(), validRegisterInput); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    strings.ToUpper(validRegisterInput.Email),
		Password: validRegisterInput.Password,
	})
	if err != nil {
		t.Errorf("Login with uppercase email: %v", err)
	}
}

// --- Refresh ---

func TestRefresh_HappyPath(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	pair, err := svc.Register(context.Background(), validRegisterInput)
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

func TestRefresh_InvalidToken(t *testing.T) {
	t.Parallel()

	svc := newTestService(t)
	_, err := svc.Refresh(context.Background(), "not.a.token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}
