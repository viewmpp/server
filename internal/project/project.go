package project

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"path/filepath"
	"server/internal/contract"
	"server/internal/user"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	AccessPrivate   = "private"
	AccessPublic    = "public"
	AccessProtected = "protected"

	MinPasswordLength = 4
	MaxPasswordLength = 72

	publicIDBytes    = 12
	maxFileNameBytes = 255
)

var ErrNotFound = errors.New("project not found")

type Project struct {
	ID        int64
	PublicID  string
	UserID    int64
	FileName  string
	Contract  []byte
	Access    string
	Password  []byte
	CreatedAt time.Time
}

func (p *Project) IsPublic() bool {
	return p.Access == AccessPublic
}

func (p *Project) IsProtected() bool {
	return p.Access == AccessProtected
}

func (p *Project) IsShared() bool {
	return p.IsPublic() || p.IsProtected()
}

func (p *Project) PasswordMatches(plaintext string) bool {
	if len(p.Password) == 0 {
		return false
	}
	return bcrypt.CompareHashAndPassword(p.Password, []byte(plaintext)) == nil
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DemoteExpiredProtected(ctx context.Context) (int64, error) {
	query := `
		UPDATE projects SET access = $1, password_hash = NULL
		WHERE access = $2
		  AND user_id IN (
		      SELECT id FROM users
		      WHERE subscription_until IS NOT NULL AND subscription_until <= now()
		  )`

	res, err := s.db.ExecContext(ctx, query, AccessPrivate, AccessProtected)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (s *Store) CountShared(ctx context.Context, userID int64) (int, error) {
	query := `SELECT count(*) FROM projects WHERE user_id = $1 AND access IN ($2, $3)`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var quantity int

	err := s.db.QueryRowContext(ctx, query, userID, AccessPublic, AccessProtected).Scan(&quantity)
	if err != nil {
		return 0, err
	}

	return quantity, nil
}

func (s *Store) CountByUserID(ctx context.Context, userID int64) (int, error) {
	query := `SELECT count(*) FROM projects WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var quantity int

	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&quantity); err != nil {
		return 0, err
	}

	return quantity, nil
}

func (s *Store) Save(ctx context.Context, userID int64, fileName string, contract []byte) (string, error) {
	fileName = sanitizeFileName(fileName)

	publicID, err := newPublicID()
	if err != nil {
		return "", err
	}

	packed, err := compress(contract)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	_, subscribed, err := lockOwner(ctx, tx, userID)
	if err != nil {
		return "", err
	}

	if !subscribed {
		var saved int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE user_id = $1`, userID).Scan(&saved); err != nil {
			return "", err
		}
		if saved >= user.MaxSavedFree {
			return "", &user.QuotaError{Kind: user.ErrSaveLimit, Used: saved, Limit: user.MaxSavedFree}
		}
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO projects (public_id, user_id, file_name, contract)
		VALUES ($1, $2, $3, $4)`, publicID, userID, fileName, packed); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}

	return publicID, nil
}

func (s *Store) GetByPublicID(ctx context.Context, publicID string, withContract bool) (*Project, error) {
	query := `
		SELECT id, public_id, user_id, file_name, access, password_hash, created_at
		FROM projects WHERE public_id = $1`

	if withContract {
		query = `
		SELECT id, public_id, user_id, file_name, access, password_hash, created_at, contract
		FROM projects WHERE public_id = $1`
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var (
		p       Project
		packed  []byte
		targets = []any{
			&p.ID, &p.PublicID, &p.UserID, &p.FileName, &p.Access, &p.Password, &p.CreatedAt,
		}
	)

	if withContract {
		targets = append(targets, &packed)
	}

	err := s.db.QueryRowContext(ctx, query, publicID).Scan(targets...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if withContract {
		if p.Contract, err = decompress(packed); err != nil {
			return nil, err
		}
	}

	return &p, nil
}

func (s *Store) ListByUserID(ctx context.Context, userID int64, limit int) ([]*Project, error) {
	query := `
		SELECT id, public_id, user_id, file_name, access, created_at
		FROM projects WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*Project

	for rows.Next() {
		var p Project
		if err = rows.Scan(&p.ID, &p.PublicID, &p.UserID, &p.FileName, &p.Access, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}

	return out, rows.Err()
}

func (s *Store) GetAccess(ctx context.Context, publicID string, userID int64) (string, error) {
	query := `SELECT access FROM projects WHERE public_id = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var access string

	err := s.db.QueryRowContext(ctx, query, publicID, userID).Scan(&access)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	return access, nil
}

func (s *Store) SetAccess(ctx context.Context, publicID string, userID int64, access string, password []byte) error {
	return s.setAccess(ctx, publicID, userID, access, password, false)
}

func (s *Store) SetAccessKeepingPassword(ctx context.Context, publicID string, userID int64, access string) error {
	return s.setAccess(ctx, publicID, userID, access, nil, true)
}

func (s *Store) setAccess(ctx context.Context, publicID string, userID int64, access string, password []byte, keep bool) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	verified, subscribed, err := lockOwner(ctx, tx, userID)
	if err != nil {
		return err
	}

	var current string
	if err = tx.QueryRowContext(ctx, `
		SELECT access FROM projects
		WHERE public_id = $1 AND user_id = $2
		FOR UPDATE`, publicID, userID).Scan(&current); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if opensNewShare(current, access) {
		if !verified {
			return user.ErrShareUnverified
		}
		if !subscribed {
			var shared int
			if err = tx.QueryRowContext(ctx, `
				SELECT count(*) FROM projects
				WHERE user_id = $1 AND access IN ($2, $3)`, userID, AccessPublic, AccessProtected).Scan(&shared); err != nil {
				return err
			}
			if shared >= user.MaxPublicFree {
				return &user.QuotaError{Kind: user.ErrShareLimit, Used: shared, Limit: user.MaxPublicFree}
			}
		}
	}

	query := `UPDATE projects SET access = $1, password_hash = $2 WHERE public_id = $3 AND user_id = $4`
	args := []any{access, password, publicID, userID}
	if keep {
		query = `UPDATE projects SET access = $1 WHERE public_id = $2 AND user_id = $3`
		args = []any{access, publicID, userID}
	}
	if _, err = tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return tx.Commit()
}

func lockOwner(ctx context.Context, tx *sql.Tx, userID int64) (bool, bool, error) {
	var verified bool
	var subscribed bool
	err := tx.QueryRowContext(ctx, `
		SELECT verified,
		       subscription = $2 AND (subscription_until IS NULL OR subscription_until > now())
		FROM users WHERE id = $1
		FOR UPDATE`, userID, user.SubscriptionPro).Scan(&verified, &subscribed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, user.ErrUserNotFound
	}
	if err != nil {
		return false, false, err
	}

	return verified, subscribed, nil
}

func (s *Store) Delete(ctx context.Context, publicID string, userID int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM projects WHERE public_id = $1 AND user_id = $2`, publicID, userID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func newPublicID() (string, error) {
	raw := make([]byte, publicIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()

	decoded, err := io.ReadAll(io.LimitReader(zr, contract.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(decoded) > contract.MaxBytes {
		return nil, fmt.Errorf("stored contract exceeds %d bytes", contract.MaxBytes)
	}
	return decoded, nil
}

func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || !utf8.ValidString(name) {
		return ""
	}

	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)

	if len(name) <= maxFileNameBytes {
		return name
	}

	cut := maxFileNameBytes
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}

	return name[:cut]
}
