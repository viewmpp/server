package billing

import "time"

const (
	RenewalGrace = 72 * time.Hour
	RetryWindow  = 15 * 24 * time.Hour
)

var statuses = map[string]bool{
	"on_trial":  true,
	"active":    true,
	"paused":    true,
	"past_due":  true,
	"unpaid":    true,
	"cancelled": true,
	"expired":   true,
}

func KnownStatus(status string) bool {
	return statuses[status]
}

func AccessUntil(e Event, now time.Time) time.Time {
	switch e.Status {
	case "on_trial":
		return after(e.TrialEndsAt, RenewalGrace, now)
	case "active":
		return after(e.RenewsAt, RenewalGrace, now)
	case "past_due":
		return after(e.RenewsAt, RetryWindow, now)
	case "cancelled":
		if e.EndsAt != nil {
			return *e.EndsAt
		}
		return now
	default:
		return now
	}
}

func after(at *time.Time, grace time.Duration, now time.Time) time.Time {
	if at == nil {
		return now
	}
	return at.Add(grace)
}

func PeriodEnd(e Event) *time.Time {
	switch e.Status {
	case "on_trial":
		return e.TrialEndsAt
	case "active", "past_due":
		return e.RenewsAt
	case "cancelled":
		return e.EndsAt
	default:
		return nil
	}
}
