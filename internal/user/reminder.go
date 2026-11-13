package user

import (
	"context"
	"log/slog"
	"server/internal/events"
	"time"
)

const (
	ReminderLead  = 24 * time.Hour
	reminderBatch = 50
)

type trialMailer interface {
	SendTrialEnding(email string, endsAt time.Time) error
}

type counter interface {
	Count(name events.Name)
}

type Reminders struct {
	store  *Store
	mailer trialMailer
	only   []int64
	counts counter
	logger *slog.Logger
}

func NewReminders(store *Store, mailer trialMailer, only []int64, counts counter, logger *slog.Logger) *Reminders {
	return &Reminders{store: store, mailer: mailer, only: only, counts: counts, logger: logger}
}

func (r *Reminders) Send(ctx context.Context) (int64, error) {
	due, err := r.store.ClaimReminders(ctx, ReminderLead, r.only, reminderBatch)
	if err != nil {
		return 0, err
	}

	var sent int64
	for _, d := range due {
		if err = r.mailer.SendTrialEnding(d.Email, d.EndsAt); err != nil {
			r.logger.Error("trial reminder not sent", "user_id", d.UserID, "err", err)
			continue
		}
		sent++
		r.counts.Count(events.TrialReminder)
	}

	return sent, nil
}
