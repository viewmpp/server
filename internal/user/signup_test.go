package user

import (
	"context"
	"errors"
	"testing"
)

type duplicateSignupStore struct {
	calls int
}

func (s *duplicateSignupStore) Save(context.Context, *User) error {
	s.calls++
	return ErrDuplicateEmail
}

func TestDuplicateSignupStopsAtTheExistingAccount(t *testing.T) {
	store := &duplicateSignupStore{}
	u := &User{Email: "owner@example.com"}

	err := createSignup(context.Background(), store, u)

	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("createSignup() error = %v, want ErrEmailTaken", err)
	}
	if store.calls != 1 {
		t.Fatalf("Save() called %d times, want 1", store.calls)
	}
	if u.ID != 0 || u.Version != 0 {
		t.Fatalf("duplicate signup claimed user ID %d version %d", u.ID, u.Version)
	}
}

type successfulSignupStore struct {
	id int64
}

func (s successfulSignupStore) Save(_ context.Context, u *User) error {
	u.ID = s.id
	return nil
}

func TestNewSignupKeepsTheCreatedIdentity(t *testing.T) {
	u := &User{Email: "new@example.com"}

	err := createSignup(context.Background(), successfulSignupStore{id: 42}, u)

	if err != nil {
		t.Fatalf("createSignup() error = %v", err)
	}
	if u.ID != 42 {
		t.Fatalf("created user ID = %d, want 42", u.ID)
	}
}
