package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrTrialUsed = errors.New("the free trial has been used")

type PaidPlan struct {
	Status    string
	Yearly    bool
	PeriodEnd *time.Time
}

type Billing interface {
	Offered(userID int64) bool
	Plan(ctx context.Context, userID int64) (*PaidPlan, error)
	Cancel(ctx context.Context, userID int64) error
}

var offer struct {
	billing   func(userID int64) bool
	trialDays int
}

func OfferBilling(offered func(userID int64) bool, trial time.Duration) {
	offer.billing = offered
	offer.trialDays = TrialDays(trial)
}

func TrialDays(trial time.Duration) int {
	return int((trial + 12*time.Hour) / (24 * time.Hour))
}

func (s *Store) StartTrial(ctx context.Context, userID int64, until time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var verified, active bool
	err = tx.QueryRowContext(ctx, `
		SELECT verified,
		       subscription = $2 AND (subscription_until IS NULL OR subscription_until > now())
		FROM users WHERE id = $1
		FOR UPDATE`, userID, SubscriptionPro).Scan(&verified, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if !verified || active {
		return ErrEditConflict
	}

	var paid bool
	if err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM billing_subscriptions WHERE user_id = $1)`, userID).Scan(&paid); err != nil {
		return err
	}
	if paid {
		return ErrTrialUsed
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO trials (user_id, ends_at) VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`, userID, until)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrTrialUsed
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE users
		SET subscription = $1, subscription_until = $2, version = version + 1
		WHERE id = $3`, SubscriptionPro, until, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) TrialOpen(ctx context.Context, userID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var open bool
	err := s.db.QueryRowContext(ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM trials WHERE user_id = $1)
		   AND NOT EXISTS (SELECT 1 FROM billing_subscriptions WHERE user_id = $1)`, userID).Scan(&open)
	return open, err
}

type Reminder struct {
	UserID int64
	Email  string
	EndsAt time.Time
}

func (s *Store) ClaimReminders(ctx context.Context, lead time.Duration, only []int64, limit int) ([]Reminder, error) {
	rows, err := s.db.QueryContext(ctx, `
		UPDATE trials
		SET reminded_at = now()
		WHERE user_id IN (
		    SELECT t.user_id
		    FROM trials t
		    JOIN users u ON u.id = t.user_id
		    WHERE t.reminded_at IS NULL
		      AND t.ends_at > now()
		      AND t.ends_at <= now() + make_interval(secs => $1)
		      AND u.verified
		      AND u.subscription = $2
		      AND u.subscription_until IS NOT NULL
		      AND u.subscription_until <= t.ends_at
		      AND ($3::bigint[] IS NULL OR t.user_id = ANY ($3::bigint[]))
		    ORDER BY t.ends_at
		    LIMIT $4
		    FOR UPDATE OF t SKIP LOCKED)
		RETURNING user_id, ends_at, (SELECT email FROM users WHERE users.id = trials.user_id)`,
		lead.Seconds(), SubscriptionPro, only, limit)
	if err != nil {
		return nil, fmt.Errorf("claim trial reminders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var due []Reminder
	for rows.Next() {
		var r Reminder
		if err = rows.Scan(&r.UserID, &r.EndsAt, &r.Email); err != nil {
			return nil, err
		}
		due = append(due, r)
	}

	return due, rows.Err()
}
