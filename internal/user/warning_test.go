package user

import (
	"testing"
	"time"
)

func TestProWarning(t *testing.T) {
	now := time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)

	at := func(d time.Duration) *User {
		until := now.Add(d)
		return &User{Verified: true, Subscription: SubscriptionPro, SubscriptionUntil: &until}
	}

	cases := []struct {
		name string
		user *User
		want string
	}{
		{"a month left", at(30 * 24 * time.Hour), ""},
		{"eight days left", at(8 * 24 * time.Hour), ""},
		{"six days left", at(6*24*time.Hour + time.Hour), "ends in 6 days"},
		{"five days left", at(5*24*time.Hour + time.Hour), "ends in 5 days"},
		{"two days left", at(2*24*time.Hour + time.Hour), "ends in 2 days"},
		{"tomorrow", at(25 * time.Hour), "ends tomorrow"},
		{"hours left", at(3 * time.Hour), "ends today"},
		{"hours left across midnight", at(16 * time.Hour), "ends tomorrow"},
		{"already lapsed", at(-time.Hour), ""},
		{"free account", &User{Verified: true, Subscription: SubscriptionFree}, ""},
		{"open ended pro", &User{Verified: true, Subscription: SubscriptionPro}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := proWarning(tc.user, now); got != tc.want {
				t.Errorf("proWarning() = %q, want %q", got, tc.want)
			}
		})
	}
}
