package project

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"time"
)

const (
	AccessPrivate = "private"
	AccessPublic  = "public"

	publicIDBytes = 12
	maxContract   = 32 << 20
)

var ErrNotFound = errors.New("project not found")

type Project struct {
	ID        int64
	PublicID  string
	UserID    int64
	FileName  string
	Contract  []byte
	Access    string
	CreatedAt time.Time
}

func (p *Project) IsPublic() bool {
	return p.Access == AccessPublic
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, userID int64, fileName string, contract []byte) (string, error) {
	publicID, err := newPublicID()
	if err != nil {
		return "", err
	}

	packed, err := compress(contract)
	if err != nil {
		return "", err
	}

	query := `
		INSERT INTO projects (public_id, user_id, file_name, contract)
		VALUES ($1, $2, $3, $4)`

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err = s.db.ExecContext(ctx, query, publicID, userID, fileName, packed); err != nil {
		return "", err
	}

	return publicID, nil
}

func (s *Store) GetByPublicID(ctx context.Context, publicID string) (*Project, error) {
	query := `
		SELECT id, public_id, user_id, file_name, contract, access, created_at
		FROM projects WHERE public_id = $1`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var (
		p      Project
		packed []byte
	)

	err := s.db.QueryRowContext(ctx, query, publicID).Scan(
		&p.ID, &p.PublicID, &p.UserID, &p.FileName, &packed, &p.Access, &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if p.Contract, err = decompress(packed); err != nil {
		return nil, err
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

func (s *Store) SetAccess(ctx context.Context, publicID string, userID int64, access string) error {
	query := `UPDATE projects SET access = $1 WHERE public_id = $2 AND user_id = $3`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, query, access, publicID, userID)
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

	return io.ReadAll(io.LimitReader(zr, maxContract))
}
