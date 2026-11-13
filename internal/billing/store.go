package billing

import (
	"context"
	"database/sql"
	"errors"
	"server/internal/user"
	"time"
)

type Outcome string

const (
	Applied    Outcome = "applied"
	Stale      Outcome = "stale"
	Superseded Outcome = "superseded"
	NoUser     Outcome = "no_user"
)

var ErrUnknownSubscription = errors.New("subscription is not linked to a user yet")

type Subscription struct {
	UserID         int64
	SubscriptionID string
	VariantID      string
	Status         string
	AccessUntil    time.Time
	PeriodEnd      *time.Time
	TestMode       bool
}

func (s *Subscription) Live(now time.Time) bool {
	return s.AccessUntil.After(now) && s.Status != "expired"
}

func (s *Subscription) Open() bool {
	switch s.Status {
	case "cancelled", "expired":
		return false
	}
	return true
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const storeTimeout = 3 * time.Second

func (s *Store) Find(ctx context.Context, userID int64) (*Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()

	sub := &Subscription{UserID: userID}
	err := s.db.QueryRowContext(ctx, `
		SELECT subscription_id, variant_id, status, access_until, period_end, test_mode
		FROM billing_subscriptions WHERE user_id = $1`, userID).
		Scan(&sub.SubscriptionID, &sub.VariantID, &sub.Status, &sub.AccessUntil, &sub.PeriodEnd, &sub.TestMode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return sub, nil
}

type current struct {
	subscriptionID string
	accessUntil    time.Time
	priorUntil     *time.Time
	eventAt        time.Time
	testMode       bool
}

func (s *Store) Apply(ctx context.Context, e Event, access time.Time) (Outcome, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = tx.Rollback() }()

	userID := e.UserID
	var linked int64
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM billing_subscriptions WHERE subscription_id = $1`, e.SubscriptionID).Scan(&linked)
	switch {
	case err == nil:
		userID = linked
	case !errors.Is(err, sql.ErrNoRows):
		return "", 0, err
	}
	if userID == 0 {
		return "", 0, ErrUnknownSubscription
	}

	var plan user.Subscription
	var until *time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT subscription, subscription_until FROM users WHERE id = $1 FOR UPDATE`, userID).
		Scan(&plan, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return NoUser, userID, nil
	}
	if err != nil {
		return "", 0, err
	}

	existing := &current{}
	err = tx.QueryRowContext(ctx, `
		SELECT subscription_id, access_until, prior_until, event_at, test_mode
		FROM billing_subscriptions WHERE user_id = $1 FOR UPDATE`, userID).
		Scan(&existing.subscriptionID, &existing.accessUntil, &existing.priorUntil, &existing.eventAt, &existing.testMode)
	if errors.Is(err, sql.ErrNoRows) {
		existing = nil
	} else if err != nil {
		return "", 0, err
	}

	var prior *time.Time

	switch {
	case existing == nil:
		if plan == user.SubscriptionPro && until != nil && until.After(time.Now()) {
			prior = until
		}
	case existing.subscriptionID == e.SubscriptionID:
		if !e.UpdatedAt.After(existing.eventAt) {
			return Stale, userID, nil
		}
		prior = existing.priorUntil
	case existing.testMode != e.TestMode:
		prior = existing.priorUntil
	default:
		if !access.After(existing.accessUntil) {
			return Superseded, userID, nil
		}
		prior = existing.priorUntil
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO billing_subscriptions
		    (user_id, subscription_id, customer_id, variant_id, status, access_until, period_end, prior_until, event_at, test_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id) DO UPDATE
		SET subscription_id = EXCLUDED.subscription_id,
		    customer_id     = EXCLUDED.customer_id,
		    variant_id      = EXCLUDED.variant_id,
		    status          = EXCLUDED.status,
		    access_until    = EXCLUDED.access_until,
		    period_end      = EXCLUDED.period_end,
		    prior_until     = EXCLUDED.prior_until,
		    event_at        = EXCLUDED.event_at,
		    test_mode       = EXCLUDED.test_mode`,
		userID, e.SubscriptionID, e.CustomerID, e.VariantID, e.Status, access, PeriodEnd(e), prior, e.UpdatedAt, e.TestMode); err != nil {
		return "", 0, err
	}

	lifetime := plan == user.SubscriptionPro && until == nil
	if !lifetime {
		effective := access
		if prior != nil && prior.After(effective) {
			effective = *prior
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE users
			SET subscription = $1, subscription_until = $2, version = version + 1
			WHERE id = $3`, user.SubscriptionPro, effective, userID); err != nil {
			return "", 0, err
		}
	}

	if err = tx.Commit(); err != nil {
		return "", 0, err
	}

	return Applied, userID, nil
}
