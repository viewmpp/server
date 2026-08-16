package user

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"server/internal/token"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateEmail = errors.New("duplicate email")
	ErrUserNotFound   = errors.New("user not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Subscription string

const (
	SubscriptionFree = Subscription("free")
	SubscriptionPro  = Subscription("pro")
)

const (
	MaxUploadFree = 10 << 20
	MaxUploadPro  = 50 << 20
)

const MaxPublicFree = 2

const MaxSavedFree = 20

var AnonymousUser = &User{Subscription: SubscriptionFree}

type User struct {
	ID                int64  `json:"id"`
	Email             string `json:"email"`
	password          `json:"-"`
	Verified          bool `json:"verified"`
	Subscription      `json:"subscription"`
	SubscriptionUntil *time.Time `json:"subscription_until"`
	CreatedAt         time.Time  `json:"created_at"`
	Version           int        `json:"version"`
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (u *User) HasSubscription() bool {
	if u.Subscription != SubscriptionPro {
		return false
	}
	return u.SubscriptionUntil == nil || u.SubscriptionUntil.After(time.Now())
}

func (u *User) CanShare(shared int) bool {
	if !u.Verified {
		return false
	}
	if u.HasSubscription() {
		return true
	}
	return shared < MaxPublicFree
}

func (u *User) CanSave(saved int) bool {
	if u.HasSubscription() {
		return true
	}
	return saved < MaxSavedFree
}

func (u *User) MaxUploadBytes() int64 {
	if u.HasSubscription() {
		return MaxUploadPro
	}
	return MaxUploadFree
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintext string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintext
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintext string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintext))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, user *User) error {
	query := `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, verified, subscription, subscription_until, created_at, version`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{user.Email, user.password.hash}

	err := s.db.QueryRowContext(ctx, query, args...,
	).Scan(
		&user.ID,
		&user.Verified,
		&user.Subscription,
		&user.SubscriptionUntil,
		&user.CreatedAt,
		&user.Version,
	)

	if err != nil {
		pgErr, exists := errors.AsType[*pgconn.PgError](err)
		if exists && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	return nil
}

func (s *Store) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET email = $1, password_hash = $2, verified = $3, version = version + 1
		WHERE id = $4 AND version = $5
		RETURNING version`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{user.Email, user.password.hash, user.Verified, user.ID, user.Version}

	err := s.db.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		pgErr, exists := errors.AsType[*pgconn.PgError](err)
		if exists && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEditConflict
		}
		return err
	}

	return nil
}

func (s *Store) CountSubscribers(ctx context.Context) (int, error) {
	query := `SELECT count(*) FROM users WHERE subscription = $1`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var count int
	if err := s.db.QueryRowContext(ctx, query, SubscriptionPro).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (s *Store) GrantSubscription(ctx context.Context, userID int64, until *time.Time) error {
	query := `
		UPDATE users
		SET subscription = $1, subscription_until = $2, version = version + 1
		WHERE id = $3 AND verified = TRUE AND subscription = $4`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, query, SubscriptionPro, until, userID, SubscriptionFree)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrEditConflict
	}

	return nil
}

func (s *Store) GetByToken(ctx context.Context, plaintext string, scope token.Scope) (*User, error) {
	tokenHash := sha256.Sum256([]byte(plaintext))

	query := `
		SELECT users.id, users.email, users.password_hash, users.verified,
		       users.subscription, users.subscription_until, users.created_at, users.version
		FROM users
		INNER JOIN tokens ON users.id = tokens.user_id
		WHERE tokens.token_hash = $1
		  AND tokens.scope = $2
		  AND tokens.expires_at > now()`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User

	err := s.db.QueryRowContext(ctx, query, tokenHash[:], scope).Scan(
		&user.ID,
		&user.Email,
		&user.password.hash,
		&user.Verified,
		&user.Subscription,
		&user.SubscriptionUntil,
		&user.CreatedAt,
		&user.Version,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (s *Store) GetByEmail(ctx context.Context, email string) (*User, error) {
	query :=
		`SELECT id, email, password_hash, verified, subscription, subscription_until, created_at, version
		FROM users WHERE email = $1`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.password.hash,
		&user.Verified,
		&user.Subscription,
		&user.SubscriptionUntil,
		&user.CreatedAt,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrUserNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (*User, error) {
	query :=
		`SELECT id, email, password_hash, verified, subscription, subscription_until, created_at, version
		FROM users WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.password.hash,
		&user.Verified,
		&user.Subscription,
		&user.SubscriptionUntil,
		&user.CreatedAt,
		&user.Version,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}
