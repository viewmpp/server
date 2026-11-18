package billing

import (
	"testing"
	"time"
)

func TestEachPaddleStatusOpensProForAsLongAsItShould(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	next := now.Add(30 * 24 * time.Hour)
	cancel := now.Add(10 * 24 * time.Hour)

	tests := []struct {
		name   string
		event  Event
		status string
		until  time.Time
	}{
		{"active", Event{Status: "active", NextBilledAt: &next}, StatusActive, next.Add(RenewalGrace)},
		{"trialing", Event{Status: "trialing", NextBilledAt: &next}, StatusTrial, next.Add(RenewalGrace)},
		{"past due", Event{Status: "past_due", NextBilledAt: &next}, StatusPastDue, next.Add(RetryWindow)},
		{"cancellation scheduled", Event{Status: "active", CancelAt: &cancel}, StatusCancelled, cancel},
		{"canceled", Event{Status: "canceled"}, StatusExpired, now},
		{"paused", Event{Status: "paused"}, StatusPaused, now},
		{"active without a next bill, from the period", Event{Status: "active", PeriodEndsAt: &next}, StatusActive, next.Add(RenewalGrace)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Status(tt.event); got != tt.status {
				t.Errorf("status = %s, want %s", got, tt.status)
			}
			if got := AccessUntil(tt.event, now); !got.Equal(tt.until) {
				t.Errorf("access until %v, want %v", got, tt.until)
			}
		})
	}
}

func TestAMissingDateNeverOpensProForever(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	for _, status := range []string{"active", "trialing", "past_due"} {
		if got := AccessUntil(Event{Status: status}, now); !got.Equal(now) {
			t.Errorf("%s without dates gives access until %v, want now", status, got)
		}
	}
}

func TestOnlyPaddleStatusesWeMapAreKnown(t *testing.T) {
	for _, s := range []string{"active", "trialing", "past_due", "paused", "canceled"} {
		if !KnownStatus(s) {
			t.Errorf("%s is a documented Paddle status", s)
		}
	}
	if KnownStatus("cancelled") || KnownStatus("refunded") || KnownStatus("") {
		t.Error("a status Paddle does not send was accepted; it would close Pro by default")
	}
}

func TestTheAccountPageShowsTheRenewalOrTheLastDay(t *testing.T) {
	next := time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)
	cancel := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	if got := PeriodEnd(Event{Status: "active", NextBilledAt: &next}); got == nil || !got.Equal(next) {
		t.Errorf("active: %v", got)
	}
	if got := PeriodEnd(Event{Status: "active", CancelAt: &cancel}); got == nil || !got.Equal(cancel) {
		t.Errorf("cancelling: %v", got)
	}
	if got := PeriodEnd(Event{Status: "canceled"}); got != nil {
		t.Errorf("canceled: %v", got)
	}
}
