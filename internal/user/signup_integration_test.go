package user

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"server/internal/config"
	"server/internal/postgres"
	"server/internal/session"
	"server/internal/token"
	"strings"
	"testing"
	"time"
)

func TestSecondSignupCannotClaimUnverifiedAccount(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set")
	}

	db, err := postgres.Open(config.DB{
		DBDSN:        dsn,
		MaxOpenConns: 4,
		MaxIdleConns: 4,
		MaxIdleTime:  time.Minute,
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

	ctx := context.Background()
	users := NewStore(db)
	tokens := token.NewStore(db)
	sessions := session.NewStore(db, time.Hour, false, "integration-test-secret", nil)

	suffix := randomTestSuffix(t)
	ownerPassword := "owner-password-" + suffix
	attackerPassword := "attacker-password-" + suffix

	owner := &User{Email: "unverified-owner-" + suffix + "@example.com"}
	if err = owner.password.Set(ownerPassword); err != nil {
		t.Fatalf("hash owner password: %v", err)
	}
	if err = users.Save(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() {
		if err := users.Delete(context.Background(), owner.ID); err != nil && !errors.Is(err, ErrUserNotFound) {
			t.Errorf("delete test owner: %v", err)
		}
	})

	verification, err := token.NewVerification(owner.ID, time.Hour)
	if err != nil {
		t.Fatalf("create verification token: %v", err)
	}
	if err = tokens.CreateVerification(ctx, verification); err != nil {
		t.Fatalf("store verification token: %v", err)
	}

	ownerID := owner.ID
	sessionToken := "owner-session-" + suffix
	if err = sessions.Save(ctx, &session.Session{
		Token:     sessionToken,
		UserID:    &ownerID,
		Data:      map[string]string{"owner": suffix},
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("store owner session: %v", err)
	}

	contract := []byte(`{"contract_version":1,"owner":"` + suffix + `"}`)
	publicID := "private-project-" + suffix
	if _, err = db.ExecContext(ctx, `
		INSERT INTO projects (public_id, user_id, file_name, contract)
		VALUES ($1, $2, $3, $4)`, publicID, owner.ID, "private.mpp", contract); err != nil {
		t.Fatalf("store private project: %v", err)
	}

	before, err := users.GetByEmail(ctx, owner.Email)
	if err != nil {
		t.Fatalf("read owner before attack: %v", err)
	}

	attacker := &User{Email: owner.Email}
	if err = attacker.password.Set(attackerPassword); err != nil {
		t.Fatalf("hash attacker password: %v", err)
	}

	err = createSignup(ctx, users, attacker)
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("second signup error = %v, want ErrEmailTaken", err)
	}
	if attacker.ID != 0 || attacker.Version != 0 {
		t.Fatalf("second signup received owner identity %d version %d", attacker.ID, attacker.Version)
	}

	after, err := users.GetByEmail(ctx, owner.Email)
	if err != nil {
		t.Fatalf("read owner after attack: %v", err)
	}
	if after.ID != before.ID || after.Version != before.Version {
		t.Fatalf("owner identity changed from %d/%d to %d/%d", before.ID, before.Version, after.ID, after.Version)
	}
	if !bytes.Equal(after.password.hash, before.password.hash) {
		t.Fatal("owner password hash changed")
	}
	if matched, err := after.password.Matches(ownerPassword); err != nil || !matched {
		t.Fatalf("owner password no longer works: matched=%v err=%v", matched, err)
	}
	if matched, err := after.password.Matches(attackerPassword); err != nil || matched {
		t.Fatalf("attacker password accepted: matched=%v err=%v", matched, err)
	}

	verifiedOwner, err := users.GetByToken(ctx, verification.Plaintext, token.ScopeVerification)
	if err != nil {
		t.Fatalf("owner verification token was removed: %v", err)
	}
	if verifiedOwner.ID != owner.ID {
		t.Fatalf("verification token resolves to user %d, want %d", verifiedOwner.ID, owner.ID)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	foundSession, err := sessions.Find(ctx, httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("owner session was removed: %v", err)
	}
	if foundSession.UserID == nil || *foundSession.UserID != owner.ID {
		t.Fatalf("owner session belongs to %v, want %d", foundSession.UserID, owner.ID)
	}

	var projectOwner int64
	var projectAccess string
	var projectContract []byte
	if err = db.QueryRowContext(ctx, `
		SELECT user_id, access, contract FROM projects WHERE public_id = $1`, publicID,
	).Scan(&projectOwner, &projectAccess, &projectContract); err != nil {
		t.Fatalf("owner project was removed: %v", err)
	}
	if projectOwner != owner.ID || projectAccess != "private" {
		t.Fatalf("private project owner/access = %d/%q, want %d/%q", projectOwner, projectAccess, owner.ID, "private")
	}
	if !bytes.Equal(projectContract, contract) {
		t.Fatal("private project contract changed")
	}
}

func randomTestSuffix(t *testing.T) string {
	t.Helper()

	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("random test suffix: %v", err)
	}
	return hex.EncodeToString(raw)
}
