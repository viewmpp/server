package session

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"server/internal/config"
	"server/internal/postgres"
	"strings"
	"testing"
	"time"
)

func TestRenewReplacesSessionAtomically(t *testing.T) {
	db := openRenewTestDB(t)
	store := NewStore(db, time.Hour, false, "renew-integration-secret", nil)
	userID := createRenewTestUser(t, db)
	sess := createRenewTestSession(t, store)

	previousToken := sess.Token
	previousCSRF := sess.csrf
	w := httptest.NewRecorder()
	if err := store.Renew(context.Background(), w, sess, &userID); err != nil {
		t.Fatalf("renew session: %v", err)
	}

	if sess.Token == previousToken || sess.csrf == previousCSRF {
		t.Fatal("renewal retained an old security value")
	}
	if sess.UserID == nil || *sess.UserID != userID {
		t.Fatalf("renewed user=%v, want %d", sess.UserID, userID)
	}
	if !sess.sent || !sess.stored || sess.Unsaved() {
		t.Fatal("renewed session is not marked as sent and stored")
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("renewal cookies=%d, want 2", len(cookies))
	}

	if _, err := findRenewTestSession(store, previousToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old token lookup error=%v, want ErrNotFound", err)
	}
	renewed, err := findRenewTestSession(store, sess.Token)
	if err != nil {
		t.Fatalf("find renewed session: %v", err)
	}
	if renewed.UserID == nil || *renewed.UserID != userID {
		t.Fatalf("stored renewed user=%v, want %d", renewed.UserID, userID)
	}
}

func TestRenewFailurePreservesOldSessionAndIdentity(t *testing.T) {
	db := openRenewTestDB(t)
	store := NewStore(db, time.Hour, false, "renew-integration-secret", nil)
	sess := createRenewTestSession(t, store)
	missingUserID := createRenewTestUser(t, db)
	if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, missingUserID); err != nil {
		t.Fatalf("remove target user: %v", err)
	}

	previousToken := sess.Token
	previousExpiry := sess.ExpiresAt
	previousCSRF := sess.csrf
	previousSent := sess.sent
	previousStored := sess.stored
	w := httptest.NewRecorder()

	if err := store.Renew(context.Background(), w, sess, &missingUserID); err == nil {
		t.Fatal("renewal with a missing user succeeded")
	}

	if sess.Token != previousToken || sess.ExpiresAt != previousExpiry || sess.csrf != previousCSRF {
		t.Fatal("failed renewal changed token, expiry, or csrf state")
	}
	if sess.UserID != nil {
		t.Fatalf("failed renewal attached user %d", *sess.UserID)
	}
	if sess.sent != previousSent || sess.stored != previousStored || sess.Unsaved() {
		t.Fatal("failed renewal changed middleware persistence state")
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("failed renewal emitted cookies")
	}

	found, err := findRenewTestSession(store, previousToken)
	if err != nil {
		t.Fatalf("old session was not rolled back: %v", err)
	}
	if found.UserID != nil || found.Data["state"] != "old" {
		t.Fatalf("old session changed: user=%v data=%v", found.UserID, found.Data)
	}
}

func openRenewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set")
	}

	db, err := postgres.Open(config.DB{
		DBDSN:          dsn,
		DBMaxOpenConns: 4,
		DBMaxIdleConns: 4,
		DBMaxIdleTime:  time.Minute,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var databaseName string
	if err = db.QueryRow(`SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Fatalf("TEST_DB_DSN points to non-test database %q", databaseName)
	}

	return db
}

func createRenewTestUser(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	var id int64
	suffix := renewTestSuffix(t)
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2) RETURNING id`, "renew-"+suffix+"@example.com", []byte("hash")).Scan(&id); err != nil {
		t.Fatalf("create renew user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("delete renew user: %v", err)
		}
	})
	return id
}

func createRenewTestSession(t *testing.T, store *Store) *Session {
	t.Helper()

	sess := &Session{
		Token:     "old-session-" + renewTestSuffix(t),
		Data:      map[string]string{"state": "old"},
		ExpiresAt: time.Now().UTC().Truncate(time.Second).Add(time.Hour),
		store:     store,
		sent:      true,
		csrf:      "old-csrf-" + renewTestSuffix(t),
	}
	if err := store.Save(context.Background(), sess); err != nil {
		t.Fatalf("save old session: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), sess.Token) })
	return sess
}

func findRenewTestSession(store *Store, token string) (*Session, error) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: store.name(), Value: token})
	return store.Find(context.Background(), httptest.NewRecorder(), r)
}

func renewTestSuffix(t *testing.T) string {
	t.Helper()

	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(raw)
}
