package user

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"server/internal/config"
	"server/internal/postgres"
	"strings"
	"testing"
	"time"
)

func TestConcurrentClaimsCannotExceedSeatQuota(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	ctx := context.Background()
	store := NewStore(db)

	baseline, err := store.CountSubscribers(ctx)
	if err != nil {
		t.Fatalf("count baseline subscribers: %v", err)
	}

	const competitors = 8
	userIDs := make([]int64, competitors)
	for i := range userIDs {
		userIDs[i] = createSeatQuotaUser(t, db, i)
	}

	until := time.Now().Add(24 * time.Hour)
	start := make(chan struct{})
	results := make(chan error, competitors)
	for _, userID := range userIDs {
		go func(userID int64) {
			<-start
			_, err := store.GrantSubscription(ctx, userID, &until, baseline+1)
			results <- err
		}(userID)
	}
	close(start)

	var succeeded int
	var refused int
	for i := 0; i < competitors; i++ {
		err = <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrSeatLimit):
			var quota *QuotaError
			if !errors.As(err, &quota) || quota.Used != baseline+1 || quota.Limit != baseline+1 {
				t.Fatalf("seat quota error=%v, want %d of %d", err, baseline+1, baseline+1)
			}
			refused++
		default:
			t.Fatalf("concurrent seat claim: %v", err)
		}
	}
	if succeeded != 1 || refused != competitors-1 {
		t.Fatalf("seat results: succeeded=%d refused=%d", succeeded, refused)
	}

	active, err := store.CountSubscribers(ctx)
	if err != nil {
		t.Fatalf("count final subscribers: %v", err)
	}
	if active != baseline+1 {
		t.Fatalf("active subscribers=%d, want %d", active, baseline+1)
	}
}

func openSeatQuotaTestDB(t *testing.T) *sql.DB {
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

func createSeatQuotaUser(t *testing.T, db *sql.DB, ordinal int) int64 {
	t.Helper()

	suffix := randomTestSuffix(t)
	var id int64
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash, verified)
		VALUES ($1, $2, TRUE) RETURNING id`, "seat-quota-"+suffix+"@example.com", []byte("hash")).Scan(&id); err != nil {
		t.Fatalf("create seat user %d: %v", ordinal, err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("delete seat user: %v", err)
		}
	})
	return id
}
