package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	CookieName = "session"
	Lifetime   = 12 * time.Hour
)

var ErrNotFound = errors.New("session not found")

type Session struct {
	Token     string
	UserID    *int64
	Data      map[string]string
	ExpiresAt time.Time
	Dropped   bool
	store     *Store
	w         http.ResponseWriter
	sent      bool
}

func (s *Session) touch() {
	if s.sent || s.store == nil || s.w == nil {
		return
	}

	s.store.setCookie(s.w, s)
	s.sent = true
}

func (s *Session) Get(key string) string {
	return s.Data[key]
}

func (s *Session) Put(key, value string) {
	s.Data[key] = value
}

func (s *Session) Pop(key string) string {
	value := s.Data[key]
	delete(s.Data, key)
	return value
}

type Store struct {
	db     *sql.DB
	secure bool
	logger *slog.Logger
}

func NewStore(db *sql.DB, secure bool, logger *slog.Logger) *Store {
	return &Store{db: db, secure: secure, logger: logger}
}

func (s *Store) New(w http.ResponseWriter) (*Session, error) {
	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	if err != nil {
		return nil, err
	}

	return &Session{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		Data:      map[string]string{},
		ExpiresAt: time.Now().Add(Lifetime),
		store:     s,
		w:         w,
	}, nil
}

func (s *Store) Find(ctx context.Context, w http.ResponseWriter, r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil, ErrNotFound
	}

	hash := hashToken(cookie.Value)

	var (
		sess = &Session{Token: cookie.Value, store: s, w: w, sent: true}
		data []byte
	)

	query := `SELECT user_id, data, expires_at FROM sessions WHERE token_hash = $1 AND expires_at > now()`

	err = s.db.QueryRowContext(ctx, query, hash[:]).Scan(&sess.UserID, &data, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &sess.Data); err != nil {
		return nil, err
	}
	if sess.Data == nil {
		sess.Data = map[string]string{}
	}

	return sess, nil
}

func (s *Store) Save(ctx context.Context, sess *Session) error {
	return s.save(ctx, sess)
}

func (s *Store) Renew(ctx context.Context, w http.ResponseWriter, sess *Session) error {
	if err := s.Delete(ctx, sess.Token); err != nil {
		return err
	}

	raw := make([]byte, 32)
	_, err := rand.Read(raw)
	if err != nil {
		return err
	}

	sess.Token = base64.RawURLEncoding.EncodeToString(raw)
	sess.ExpiresAt = time.Now().Add(Lifetime)

	if err = s.save(ctx, sess); err != nil {
		return err
	}

	s.setCookie(w, sess)
	sess.sent = true

	return nil
}

func (s *Store) Delete(ctx context.Context, token string) error {
	hash := hashToken(token)
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash[:])
	return err
}

func (s *Store) Clear(ctx context.Context, w http.ResponseWriter, sess *Session) error {
	if err := s.Delete(ctx, sess.Token); err != nil {
		return err
	}

	sess.Dropped = true

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func (s *Store) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) save(ctx context.Context, sess *Session) error {
	data, err := json.Marshal(sess.Data)
	if err != nil {
		return err
	}

	hash := hashToken(sess.Token)

	query := `
		INSERT INTO sessions (token_hash, user_id, data, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token_hash) DO UPDATE SET user_id = $2, data = $3, expires_at = $4`

	_, err = s.db.ExecContext(ctx, query, hash[:], sess.UserID, data, sess.ExpiresAt)

	return err
}

func (s *Store) setCookie(w http.ResponseWriter, sess *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func hashToken(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func (s *Store) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}
