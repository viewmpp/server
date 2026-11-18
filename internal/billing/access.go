package billing

import "time"

const (
	RenewalGrace = 72 * time.Hour
	RetryWindow  = 15 * 24 * time.Hour
)

const (
	StatusActive    = "active"
	StatusTrial     = "on_trial"
	StatusPastDue   = "past_due"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)

var paddleStatuses = map[string]string{
	"active":   StatusActive,
	"trialing": StatusTrial,
	"past_due": StatusPastDue,
	"paused":   StatusPaused,
	"canceled": StatusExpired,
}

func KnownStatus(paddle string) bool {
	_, ok := paddleStatuses[paddle]
	return ok
}

func Status(e Event) string {
	status := paddleStatuses[e.Status]
	if e.CancelAt != nil && (status == StatusActive || status == StatusTrial || status == StatusPastDue) {
		return StatusCancelled
	}
	return status
}

func AccessUntil(e Event, now time.Time) time.Time {
	switch Status(e) {
	case StatusTrial, StatusActive:
		return after(renewal(e), RenewalGrace, now)
	case StatusPastDue:
		return after(renewal(e), RetryWindow, now)
	case StatusCancelled:
		return *e.CancelAt
	default:
		return now
	}
}

func PeriodEnd(e Event) *time.Time {
	switch Status(e) {
	case StatusTrial, StatusActive, StatusPastDue:
		return renewal(e)
	case StatusCancelled:
		return e.CancelAt
	default:
		return nil
	}
}

func renewal(e Event) *time.Time {
	if e.NextBilledAt != nil {
		return e.NextBilledAt
	}
	return e.PeriodEndsAt
}

func after(at *time.Time, grace time.Duration, now time.Time) time.Time {
	if at == nil {
		return now
	}
	return at.Add(grace)
}
