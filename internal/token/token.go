package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"
)

type Scope string

var (
	ScopeVerification = Scope("verification")
)

type Token struct {
	UserID    int64
	Plaintext string
	Hash      []byte
	Scope
	ExpiresAt time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func NewVerification(userID int64, ttl time.Duration) (*Token, error) {
	return New(userID, ScopeVerification, ttl, 16)
}

func New(userID int64, scope Scope, ttl time.Duration, bytes int) (*Token, error) {
	token := &Token{
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(ttl),
	}

	randBytes := make([]byte, bytes)

	_, err := rand.Read(randBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randBytes)

	hash := sha256.Sum256([]byte(token.Plaintext))

	token.Hash = hash[:]

	return token, nil
}

func (s *Store) CreateVerification(ctx context.Context, token *Token) error {
	query :=
		`INSERT INTO tokens (user_id, token_hash, scope, expires_at)
		VALUES ($1, $2, $3, $4) 
		ON CONFLICT (user_id) 
		WHERE scope = 'verification'
      	DO UPDATE SET token_hash = EXCLUDED.token_hash, expires_at  = EXCLUDED.expires_at`

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	args := []any{token.UserID, token.Hash, token.Scope, token.ExpiresAt}

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tokens WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) DeleteVerificationsByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM tokens WHERE user_id = $1 AND	scope = 'verification'`

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) LatestVerificationIssuedAt(ctx context.Context, userID int64, ttl time.Duration) (time.Time, bool, error) {
	query := `SELECT max(expires_at) FROM tokens WHERE user_id = $1 AND scope = 'verification'`

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	var expires sql.NullTime

	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&expires); err != nil {
		return time.Time{}, false, err
	}

	if !expires.Valid {
		return time.Time{}, false, nil
	}

	return expires.Time.Add(-ttl), true, nil
}
