package user

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

func trialUser(t *testing.T, db *sql.DB, verified bool) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash, verified)
		VALUES ($1, $2, $3) RETURNING id`, "trial-"+randomTestSuffix(t)+"@example.com", []byte("hash"), verified).Scan(&id); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, id) })
	return id
}

func TestATrialIsGivenOncePerAccount(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	store := NewStore(db)
	ctx := context.Background()
	id := trialUser(t, db, true)

	if open, err := store.TrialOpen(ctx, id); err != nil || !open {
		t.Fatalf("a new account has no trial: open=%v err=%v", open, err)
	}

	until := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Second)
	if err := store.StartTrial(ctx, id, until); err != nil {
		t.Fatalf("start trial: %v", err)
	}
	u, err := store.GetByID(ctx, id)
	if err != nil || !u.HasSubscription() || !u.SubscriptionUntil.Equal(until) {
		t.Fatalf("after the trial started: %+v, %v", u, err)
	}

	if _, err = db.Exec(`UPDATE users SET subscription_until = now() - interval '1 hour' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if err = store.StartTrial(ctx, id, until); !errors.Is(err, ErrTrialUsed) {
		t.Errorf("a second trial after the first ran out: err = %v", err)
	}
	if open, _ := store.TrialOpen(ctx, id); open {
		t.Error("the trial still shows as open after it was used")
	}
}

func TestTwoClicksAtOnceStartOneTrial(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	store := NewStore(db)
	id := trialUser(t, db, true)
	until := time.Now().Add(7 * 24 * time.Hour)

	const clicks = 6
	errs := make(chan error, clicks)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range clicks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- store.StartTrial(context.Background(), id, until)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	started := 0
	for err := range errs {
		switch {
		case err == nil:
			started++
		case errors.Is(err, ErrTrialUsed), errors.Is(err, ErrEditConflict):
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if started != 1 {
		t.Errorf("%d trials started, want 1", started)
	}
}

func TestNoTrialForAnUnconfirmedOrPayingAccount(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	store := NewStore(db)
	ctx := context.Background()
	until := time.Now().Add(7 * 24 * time.Hour)

	unconfirmed := trialUser(t, db, false)
	if err := store.StartTrial(ctx, unconfirmed, until); !errors.Is(err, ErrEditConflict) {
		t.Errorf("unconfirmed account: err = %v", err)
	}

	paid := trialUser(t, db, true)
	if _, err := db.Exec(`
		INSERT INTO billing_subscriptions (user_id, subscription_id, customer_id, variant_id, status, access_until, event_at, test_mode)
		VALUES ($1, $2, '1', '1', 'expired', now(), now(), true)`, paid, "sub-"+randomTestSuffix(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.StartTrial(ctx, paid, until); !errors.Is(err, ErrTrialUsed) {
		t.Errorf("an account that already paid got a trial: err = %v", err)
	}
}

func TestEarlyAccessCountsAsTheTrial(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	store := NewStore(db)
	ctx := context.Background()
	id := trialUser(t, db, true)

	until := time.Now().Add(90 * 24 * time.Hour)
	if _, err := store.GrantSubscription(ctx, id, &until, 1_000_000); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if open, err := store.TrialOpen(ctx, id); err != nil || open {
		t.Errorf("early access left the trial open: open=%v err=%v", open, err)
	}
}

func TestEachEndingTrialGetsOneReminder(t *testing.T) {
	db := openSeatQuotaTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	set := func(id int64, endsIn, proIn time.Duration) {
		t.Helper()
		ends := time.Now().Add(endsIn)
		pro := time.Now().Add(proIn)
		if _, err := db.Exec(`INSERT INTO trials (user_id, ends_at) VALUES ($1, $2)`, id, ends); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE users SET subscription = 'pro', subscription_until = $2 WHERE id = $1`, id, pro); err != nil {
			t.Fatal(err)
		}
	}

	due := trialUser(t, db, true)
	set(due, 20*time.Hour, 20*time.Hour)
	paid := trialUser(t, db, true)
	set(paid, 20*time.Hour, 30*24*time.Hour)
	later := trialUser(t, db, true)
	set(later, 3*24*time.Hour, 3*24*time.Hour)
	over := trialUser(t, db, true)
	set(over, -time.Hour, -time.Hour)

	only := []int64{due, paid, later, over}

	got, err := store.ClaimReminders(ctx, 24*time.Hour, only, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UserID != due || got[0].Email == "" {
		t.Fatalf("claimed %+v, want only the trial ending within a day and not paid for", got)
	}

	again, err := store.ClaimReminders(ctx, 24*time.Hour, only, 50)
	if err != nil || len(again) != 0 {
		t.Errorf("claimed again %+v, %v: the reminder would go out twice", again, err)
	}

	outsider := trialUser(t, db, true)
	set(outsider, 20*time.Hour, 20*time.Hour)
	if got, err = store.ClaimReminders(ctx, 24*time.Hour, []int64{due}, 50); err != nil || len(got) != 0 {
		t.Errorf("in test mode a non-tester was claimed: %+v, %v", got, err)
	}

	got, err = store.ClaimReminders(ctx, 24*time.Hour, nil, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range got {
		found = found || r.UserID == outsider
	}
	if !found {
		t.Error("live reminders skipped an account that is due")
	}
}
