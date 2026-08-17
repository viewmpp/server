package user

import (
	"server/internal/assert"
	"testing"
	"time"
)

func TestCanSave(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	pro := &User{Subscription: SubscriptionPro, SubscriptionUntil: &future}
	lapsed := &User{Subscription: SubscriptionPro, SubscriptionUntil: &past}
	free := &User{Subscription: SubscriptionFree}
	unverified := &User{Subscription: SubscriptionFree, Verified: false}

	tests := []struct {
		name  string
		user  *User
		saved int
		want  bool
	}{
		{"free, empty", free, 0, true},
		{"free, one below the cap", free, MaxSavedFree - 1, true},
		{"free, at the cap", free, MaxSavedFree, false},
		{"free, above the cap", free, MaxSavedFree + 5, false},
		{"unverified is capped the same", unverified, MaxSavedFree, false},
		{"unverified may still save below it", unverified, 0, true},
		{"pro at the free cap", pro, MaxSavedFree, true},
		{"pro far above it", pro, 5000, true},
		{"lapsed pro falls back to free", lapsed, MaxSavedFree, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.user.CanSave(tc.saved), tc.want)
		})
	}
}

func TestSaveCapFitsInTheProjectList(t *testing.T) {
	const listLimit = 100

	if MaxSavedFree > listLimit {
		t.Fatalf("MaxSavedFree = %d exceeds the project list limit of %d: a free user could hold plans the list never shows, and so could never delete them", MaxSavedFree, listLimit)
	}
}
