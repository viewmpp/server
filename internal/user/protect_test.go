package user

import (
	"server/internal/assert"
	"testing"
	"time"
)

func TestCanProtect(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name string
		user User
		want bool
	}{
		{"anonymous", User{}, false},
		{"free, unverified", User{Subscription: SubscriptionFree}, false},
		{"free, verified", User{Verified: true, Subscription: SubscriptionFree}, false},
		{"pro, unverified", User{Subscription: SubscriptionPro, SubscriptionUntil: &future}, false},
		{"pro, verified", User{Verified: true, Subscription: SubscriptionPro, SubscriptionUntil: &future}, true},
		{"pro, open ended", User{Verified: true, Subscription: SubscriptionPro}, true},
		{"pro, lapsed", User{Verified: true, Subscription: SubscriptionPro, SubscriptionUntil: &past}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.user.CanProtect(), tc.want)
		})
	}
}

func TestPublicSharingStaysFree(t *testing.T) {
	free := User{Verified: true, Subscription: SubscriptionFree}

	if !free.CanShare(0) {
		t.Fatal("a verified free account cannot create a public link: that link is the growth channel and must stay free")
	}
	if free.CanProtect() {
		t.Fatal("a free account can put a password on a link")
	}
}
