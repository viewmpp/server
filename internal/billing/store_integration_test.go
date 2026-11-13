package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"server/internal/config"
	"server/internal/postgres"
	"strings"
	"sync"
	"testing"
	"time"
)

func openBillingTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN is not set")
	}

	pg, err := postgres.Open(config.DB{
		DBDSN:          dsn,
		DBMaxOpenConns: 16,
		DBMaxIdleConns: 16,
		DBMaxIdleTime:  time.Minute,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db := pg.DB
	t.Cleanup(func() { _ = db.Close() })

	var name string
	if err = db.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
		t.Fatalf("read database name: %v", err)
	}
	if !strings.Contains(strings.ToLower(name), "test") {
		t.Fatalf("TEST_DB_DSN points to non-test database %q", name)
	}

	return db
}

func randomID(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func billingUser(t *testing.T, db *sql.DB, plan string, until *time.Time) int64 {
	t.Helper()

	var id int64
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash, verified, subscription, subscription_until)
		VALUES ($1, $2, TRUE, $3, $4) RETURNING id`,
		"billing-"+randomID(t)+"@example.com", []byte("hash"), plan, until).Scan(&id); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM users WHERE id = $1`, id) })
	return id
}

func planOf(t *testing.T, db *sql.DB, userID int64) (string, *time.Time) {
	t.Helper()

	var plan string
	var until *time.Time
	if err := db.QueryRow(`SELECT subscription, subscription_until FROM users WHERE id = $1`, userID).Scan(&plan, &until); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	return plan, until
}

type scenario struct {
	t     *testing.T
	db    *sql.DB
	store *Store
	user  int64
	sub   string
	clock time.Time
}

func newScenario(t *testing.T, plan string, until *time.Time) *scenario {
	db := openBillingTestDB(t)
	return &scenario{
		t:     t,
		db:    db,
		store: NewStore(db),
		user:  billingUser(t, db, plan, until),
		sub:   "sub-" + randomID(t),
		clock: time.Now().UTC().Truncate(time.Second),
	}
}

func (s *scenario) event(status string, updatedAgo time.Duration, withUser bool) Event {
	e := Event{
		Name:           "subscription_updated",
		SubscriptionID: s.sub,
		CustomerID:     "77",
		StoreID:        "513638",
		VariantID:      "2370696",
		Status:         status,
		UpdatedAt:      s.clock.Add(-updatedAgo),
		TestMode:       true,
	}
	if withUser {
		e.UserID = s.user
	}
	return e
}

func (s *scenario) apply(e Event, access time.Time) Outcome {
	s.t.Helper()
	outcome, _, err := s.store.Apply(context.Background(), e, access)
	if err != nil {
		s.t.Fatalf("apply %s: %v", e.Status, err)
	}
	return outcome
}

func (s *scenario) wantUntil(want time.Time, why string) {
	s.t.Helper()
	plan, until := planOf(s.t, s.db, s.user)
	if plan != "pro" || until == nil || !until.Equal(want) {
		s.t.Errorf("plan = %s until %v, want pro until %v: %s", plan, until, want, why)
	}
}

func TestAPaymentOpensProAndACancellationKeepsThePaidPeriod(t *testing.T) {
	s := newScenario(t, "free", nil)

	paid := s.clock.Add(30 * 24 * time.Hour)
	if got := s.apply(s.event("active", 2*time.Minute, true), paid); got != Applied {
		t.Fatalf("outcome = %s", got)
	}
	s.wantUntil(paid, "a paid subscription opens Pro")

	ends := s.clock.Add(20 * 24 * time.Hour)
	s.apply(s.event("cancelled", time.Minute, false), ends)
	s.wantUntil(ends, "a renewal without custom data is found by its subscription")

	sub, err := s.store.Find(context.Background(), s.user)
	if err != nil || sub == nil || sub.Status != "cancelled" || !sub.Live(s.clock) {
		t.Errorf("find = %+v, %v", sub, err)
	}
}

func TestALateOrRepeatedEventDoesNotUndoANewerOne(t *testing.T) {
	s := newScenario(t, "free", nil)

	expired := s.event("expired", 0, true)
	s.apply(expired, s.clock)

	late := s.event("active", time.Hour, true)
	if got := s.apply(late, s.clock.Add(30*24*time.Hour)); got != Stale {
		t.Errorf("outcome = %s, want stale", got)
	}
	if got := s.apply(expired, s.clock); got != Stale {
		t.Errorf("a repeated delivery was applied twice: %s", got)
	}
	s.wantUntil(s.clock, "an active event older than the expiry must not reopen Pro")
}

func TestEarlyAccessIsNotCutShortByAPaidSubscription(t *testing.T) {
	early := time.Now().Add(60 * 24 * time.Hour).UTC().Truncate(time.Second)
	s := newScenario(t, "pro", &early)

	paid := s.clock.Add(30 * 24 * time.Hour)
	s.apply(s.event("active", 2*time.Minute, true), paid)
	s.wantUntil(early, "the early access that runs longer wins")

	s.apply(s.event("expired", time.Minute, false), s.clock)
	s.wantUntil(early, "ending the paid plan gives back what early access promised")
}

func TestAnOldSubscriptionCannotOverwriteANewOne(t *testing.T) {
	s := newScenario(t, "free", nil)
	old := s.sub

	s.apply(s.event("expired", time.Hour, true), s.clock)

	s.sub = "sub-" + randomID(t)
	renewed := s.clock.Add(30 * 24 * time.Hour)
	if got := s.apply(s.event("active", 30*time.Minute, true), renewed); got != Applied {
		t.Fatalf("the new subscription was not applied: %s", got)
	}

	s.sub = old
	if got := s.apply(s.event("expired", 0, true), s.clock); got != Superseded {
		t.Errorf("outcome = %s, want superseded", got)
	}
	s.wantUntil(renewed, "a late event of the old subscription must not close the new one")
}

func TestALifetimeGrantIsLeftAlone(t *testing.T) {
	s := newScenario(t, "pro", nil)

	s.apply(s.event("expired", 0, true), s.clock)

	plan, until := planOf(t, s.db, s.user)
	if plan != "pro" || until != nil {
		t.Errorf("plan = %s until %v: a lifetime grant was ended by a subscription", plan, until)
	}
}

func TestEventsForUnknownOrDeletedUsers(t *testing.T) {
	s := newScenario(t, "free", nil)

	_, _, err := s.store.Apply(context.Background(), s.event("active", 0, false), s.clock)
	if !errors.Is(err, ErrUnknownSubscription) {
		t.Errorf("err = %v, want ErrUnknownSubscription", err)
	}

	gone := s.event("active", 0, true)
	gone.UserID = s.user + 1_000_000
	if outcome, _, err := s.store.Apply(context.Background(), gone, s.clock); err != nil || outcome != NoUser {
		t.Errorf("outcome = %s, err = %v", outcome, err)
	}
}

func TestDuplicateDeliveriesAtOnceApplyOnce(t *testing.T) {
	s := newScenario(t, "free", nil)
	e := s.event("active", 0, true)
	access := s.clock.Add(30 * 24 * time.Hour)

	const deliveries = 6
	outcomes := make(chan Outcome, deliveries)
	errs := make(chan error, deliveries)
	start := make(chan struct{})

	var wg sync.WaitGroup
	for range deliveries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outcome, _, err := s.store.Apply(context.Background(), e, access)
			if err != nil {
				errs <- err
				return
			}
			outcomes <- outcome
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)
	close(errs)

	for err := range errs {
		t.Errorf("a concurrent delivery failed: %v", err)
	}
	applied := 0
	for o := range outcomes {
		if o == Applied {
			applied++
		}
	}
	if applied != 1 {
		t.Errorf("applied %d times, want once", applied)
	}
	s.wantUntil(access, "")
}

func TestARealPurchaseReplacesALongerTestSubscription(t *testing.T) {
	s := newScenario(t, "free", nil)

	testYear := s.clock.Add(365 * 24 * time.Hour)
	s.apply(s.event("active", time.Hour, true), testYear)

	s.sub = "sub-" + randomID(t)
	live := s.event("active", 0, true)
	live.TestMode = false
	month := s.clock.Add(30 * 24 * time.Hour)
	if got := s.apply(live, month); got != Applied {
		t.Fatalf("outcome = %s: a paid subscription was ignored because a test one ran longer", got)
	}
	s.wantUntil(month, "after launch the real subscription decides")

	sub, err := s.store.Find(context.Background(), s.user)
	if err != nil || sub == nil || sub.TestMode || sub.SubscriptionID != s.sub {
		t.Errorf("find = %+v, %v", sub, err)
	}
}

func TestTheRenewalDateIsKeptForTheAccountPage(t *testing.T) {
	s := newScenario(t, "free", nil)

	renews := s.clock.Add(30 * 24 * time.Hour)
	e := s.event("active", 0, true)
	e.RenewsAt = &renews
	s.apply(e, AccessUntil(e, s.clock))

	sub, err := s.store.Find(context.Background(), s.user)
	if err != nil || sub == nil || sub.PeriodEnd == nil || !sub.PeriodEnd.Equal(renews) {
		t.Errorf("period end = %v, want %v (err %v)", sub, renews, err)
	}
}
