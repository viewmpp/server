package billing

import (
	"testing"
	"time"
)

func TestEachStatusOpensProForAsLongAsItShould(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	renews := now.Add(30 * 24 * time.Hour)
	ends := now.Add(10 * 24 * time.Hour)
	trial := now.Add(7 * 24 * time.Hour)

	tests := []struct {
		status string
		want   time.Time
		why    string
	}{
		{"on_trial", trial.Add(RenewalGrace), "a trial lasts until it ends, with room for the first charge"},
		{"active", renews.Add(RenewalGrace), "a paid period lasts until renewal, with room for the renewal webhook"},
		{"past_due", renews.Add(RetryWindow), "the card is retried for two weeks and the customer keeps Pro meanwhile"},
		{"cancelled", ends, "a cancelled subscription keeps what was paid for"},
		{"paused", now, "a paused subscription is not paid for"},
		{"unpaid", now, "every retry failed"},
		{"expired", now, "the subscription is over"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			e := Event{Status: tt.status, RenewsAt: &renews, EndsAt: &ends, TrialEndsAt: &trial}
			if got := AccessUntil(e, now); !got.Equal(tt.want) {
				t.Errorf("access until %v, want %v: %s", got, tt.want, tt.why)
			}
		})
	}
}

func TestAMissingDateNeverOpensProForever(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	for _, status := range []string{"on_trial", "active", "past_due", "cancelled"} {
		if got := AccessUntil(Event{Status: status}, now); !got.Equal(now) {
			t.Errorf("%s without dates gives access until %v, want now", status, got)
		}
	}
}

func TestOnlyStatusesWeMapAreKnown(t *testing.T) {
	if KnownStatus("refunded") || KnownStatus("") {
		t.Error("a status we have no rule for was accepted; it would close Pro by default")
	}
	if !KnownStatus("past_due") {
		t.Error("past_due is a documented status")
	}
}
