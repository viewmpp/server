package user

import (
	"testing"
	"time"
)

func TestCanShare(t *testing.T) {
	cases := []struct {
		name   string
		user   User
		shared int
		want   bool
	}{
		{"unverified free", User{Subscription: SubscriptionFree}, 0, false},
		{"unverified pro", User{Subscription: SubscriptionPro}, 0, false},

		{"verified free, none shared", User{Verified: true, Subscription: SubscriptionFree}, 0, true},
		{"verified free, one shared", User{Verified: true, Subscription: SubscriptionFree}, 1, true},
		{"verified free, at the limit", User{Verified: true, Subscription: SubscriptionFree}, MaxPublicFree, false},
		{"verified free, over the limit", User{Verified: true, Subscription: SubscriptionFree}, MaxPublicFree + 1, false},

		{"verified pro, at the free limit", User{Verified: true, Subscription: SubscriptionPro}, MaxPublicFree, true},
		{"verified pro, many shared", User{Verified: true, Subscription: SubscriptionPro}, 500, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.user.CanShare(c.shared); got != c.want {
				t.Errorf("CanShare(%d) = %v, want %v", c.shared, got, c.want)
			}
		})
	}
}

func TestAnonymousCannotShare(t *testing.T) {
	if AnonymousUser.CanShare(0) {
		t.Error("anonymous user may share")
	}
}

func TestHasSubscriptionRespectsExpiry(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	cases := []struct {
		name string
		user User
		want bool
	}{
		{"free", User{Subscription: SubscriptionFree}, false},
		{"pro, no expiry", User{Subscription: SubscriptionPro}, true},
		{"pro, expires later", User{Subscription: SubscriptionPro, SubscriptionUntil: &future}, true},
		{"pro, expired", User{Subscription: SubscriptionPro, SubscriptionUntil: &past}, false},
		{"free with future date", User{Subscription: SubscriptionFree, SubscriptionUntil: &future}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.user.HasSubscription(); got != c.want {
				t.Errorf("HasSubscription() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestExpiredProLosesUnlimitedSharing(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	u := User{Verified: true, Subscription: SubscriptionPro, SubscriptionUntil: &past}

	if u.CanShare(MaxPublicFree) {
		t.Error("expired subscription still grants unlimited sharing")
	}
	if !u.CanShare(0) {
		t.Error("expired subscriber lost the free allowance too")
	}
}
