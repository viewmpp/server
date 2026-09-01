package project

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"server/internal/config"
	"server/internal/postgres"
	"server/internal/user"
	"strings"
	"testing"
	"time"
)

func TestConcurrentSavesCannotExceedQuota(t *testing.T) {
	db := openProjectQuotaTestDB(t)
	ctx := context.Background()
	ownerID, suffix := createProjectQuotaUser(t, db, true)

	for i := 0; i < user.MaxSavedFree-1; i++ {
		insertQuotaProject(t, db, ownerID, fmt.Sprintf("save-existing-%s-%d", suffix, i), AccessPrivate)
	}

	store := NewStore(db)
	const competitors = 8
	start := make(chan struct{})
	results := make(chan error, competitors)
	for i := 0; i < competitors; i++ {
		go func(i int) {
			<-start
			_, err := store.Save(ctx, ownerID, fmt.Sprintf("attempt-%d.mpp", i), []byte(`{"contract_version":1}`))
			results <- err
		}(i)
	}
	close(start)

	succeeded, refused := quotaResults(t, results, competitors, user.ErrSaveLimit, user.MaxSavedFree, user.MaxSavedFree)
	if succeeded != 1 || refused != competitors-1 {
		t.Fatalf("save results: succeeded=%d refused=%d", succeeded, refused)
	}

	var saved int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE user_id = $1`, ownerID).Scan(&saved); err != nil {
		t.Fatalf("count saved projects: %v", err)
	}
	if saved != user.MaxSavedFree {
		t.Fatalf("saved projects=%d, want %d", saved, user.MaxSavedFree)
	}
}

func TestConcurrentSharesCannotExceedQuota(t *testing.T) {
	db := openProjectQuotaTestDB(t)
	ctx := context.Background()
	ownerID, suffix := createProjectQuotaUser(t, db, true)

	for i := 0; i < user.MaxPublicFree-1; i++ {
		insertQuotaProject(t, db, ownerID, fmt.Sprintf("share-existing-%s-%d", suffix, i), AccessPublic)
	}

	const competitors = 8
	targets := make([]string, competitors)
	for i := range targets {
		targets[i] = fmt.Sprintf("share-target-%s-%d", suffix, i)
		insertQuotaProject(t, db, ownerID, targets[i], AccessPrivate)
	}

	store := NewStore(db)
	start := make(chan struct{})
	results := make(chan error, competitors)
	for _, publicID := range targets {
		go func(publicID string) {
			<-start
			results <- store.SetAccess(ctx, publicID, ownerID, AccessPublic, nil)
		}(publicID)
	}
	close(start)

	succeeded, refused := quotaResults(t, results, competitors, user.ErrShareLimit, user.MaxPublicFree, user.MaxPublicFree)
	if succeeded != 1 || refused != competitors-1 {
		t.Fatalf("share results: succeeded=%d refused=%d", succeeded, refused)
	}

	var shared int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM projects
		WHERE user_id = $1 AND access IN ($2, $3)`, ownerID, AccessPublic, AccessProtected).Scan(&shared); err != nil {
		t.Fatalf("count shared projects: %v", err)
	}
	if shared != user.MaxPublicFree {
		t.Fatalf("shared projects=%d, want %d", shared, user.MaxPublicFree)
	}
}

func quotaResults(t *testing.T, results <-chan error, total int, limitError error, used, limit int) (int, int) {
	t.Helper()

	var succeeded int
	var refused int
	for i := 0; i < total; i++ {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, limitError):
			var quota *user.QuotaError
			if !errors.As(err, &quota) || quota.Used != used || quota.Limit != limit {
				t.Fatalf("quota error=%v, want %d of %d", err, used, limit)
			}
			refused++
		default:
			t.Fatalf("concurrent operation: %v", err)
		}
	}
	return succeeded, refused
}

func openProjectQuotaTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set")
	}

	db, err := postgres.Open(config.DB{
		DBDSN:        dsn,
		MaxOpenConns: 16,
		MaxIdleConns: 16,
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

	return db
}

func createProjectQuotaUser(t *testing.T, db *sql.DB, verified bool) (int64, string) {
	t.Helper()

	suffix := projectQuotaSuffix(t)
	var id int64
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash, verified)
		VALUES ($1, $2, $3) RETURNING id`, "project-quota-"+suffix+"@example.com", []byte("hash"), verified).Scan(&id); err != nil {
		t.Fatalf("create quota user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("delete quota user: %v", err)
		}
	})
	return id, suffix
}

func insertQuotaProject(t *testing.T, db *sql.DB, ownerID int64, publicID, access string) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO projects (public_id, user_id, file_name, contract, access)
		VALUES ($1, $2, $3, $4, $5)`, publicID, ownerID, "quota.mpp", []byte("contract"), access); err != nil {
		t.Fatalf("insert quota project: %v", err)
	}
}

func projectQuotaSuffix(t *testing.T) string {
	t.Helper()

	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(raw)
}
