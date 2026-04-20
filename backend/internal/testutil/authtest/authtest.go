// Package authtest provides in-memory test doubles for the auth package.
// Import this package from test files in other packages that need an auth
// service without a real database.
package authtest

import (
	"context"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/uuid"
)

// FakeStore is an in-memory implementation of auth.Store.
// It is safe for use within a single goroutine; add a mutex if parallel
// tests share the same instance.
type FakeStore struct {
	byEmail map[string]auth.User
	byID    map[uuid.UUID]auth.User
}

// NewFakeStore returns an empty FakeStore.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		byEmail: make(map[string]auth.User),
		byID:    make(map[uuid.UUID]auth.User),
	}
}

// CreateUser implements auth.Store.
func (f *FakeStore) CreateUser(_ context.Context, nu auth.NewUser) (auth.User, error) {
	if _, exists := f.byEmail[nu.Email]; exists {
		return auth.User{}, auth.ErrEmailTaken
	}
	for _, u := range f.byEmail {
		if u.Username == nu.Username {
			return auth.User{}, auth.ErrUsernameTaken
		}
	}
	id := uuid.New()
	u := auth.User{ID: id, Email: nu.Email, Username: nu.Username, PasswordHash: nu.PasswordHash}
	f.byEmail[nu.Email] = u
	f.byID[id] = u
	return u, nil
}

// GetUserByEmail implements auth.Store.
func (f *FakeStore) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return u, nil
}

// GetUserByID implements auth.Store.
func (f *FakeStore) GetUserByID(_ context.Context, id uuid.UUID) (auth.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return u, nil
}

// NewService returns an auth.Service backed by an in-memory FakeStore.
func NewService(jwtSecret []byte) *auth.Service {
	return auth.NewServiceWithStore(NewFakeStore(), jwtSecret)
}
