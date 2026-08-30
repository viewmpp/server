package user

import (
	"strings"
	"testing"
	"time"
)

func TestBoundedGrantLapsesToFree(t *testing.T) {
	const period = 30 * 24 * time.Hour

	granted := time.Now()
	until := granted.Add(period)

	pro := &User{Verified: true, Subscription: SubscriptionPro, SubscriptionUntil: &until}

	if !pro.HasSubscription() {
		t.Fatal("a fresh month-long grant is not active")
	}

	lapsed := granted.Add(-time.Minute)
	pro.SubscriptionUntil = &lapsed

	if pro.HasSubscription() {
		t.Fatal("an expired grant still counts as Pro: the whole point of the period is that it ends")
	}

	if pro.CanShare(MaxPublicFree) {
		t.Error("an expired subscriber keeps unlimited sharing")
	}
	if !pro.CanShare(0) {
		t.Error("an expired subscriber lost the free allowance too")
	}
	if pro.CanSave(MaxSavedFree) {
		t.Error("an expired subscriber keeps unlimited saving")
	}
}

func TestOpenEndedGrantStaysActive(t *testing.T) {
	forever := &User{Subscription: SubscriptionPro}

	if !forever.HasSubscription() {
		t.Fatal("a grant with no end date must stay active: accounts created before the period existed rely on it")
	}
}

func TestGrantMessageCarriesTheDate(t *testing.T) {
	until := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	got := MsgEarlyAccessGranted(7, until)

	for _, want := range []string{"first 7 users", "20 September 2026"} {
		if !strings.Contains(got, want) {
			t.Errorf("message %q does not mention %q", got, want)
		}
	}
}
