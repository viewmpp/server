package user

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"server/internal/clientip"
	"server/internal/ratelimit"
	"server/internal/session"
	"strings"
	"testing"
	"time"
)

type fakeBilling struct {
	offered   map[int64]bool
	cancelErr error
	cancelled []int64
}

func (f *fakeBilling) Offered(userID int64) bool { return f.offered[userID] }

func (f *fakeBilling) Plan(context.Context, int64) (*PaidPlan, error) { return nil, nil }

func (f *fakeBilling) Cancel(_ context.Context, userID int64) error {
	if f.cancelErr != nil {
		return f.cancelErr
	}
	f.cancelled = append(f.cancelled, userID)
	return nil
}

func TestTrialDaysRoundToWholeDays(t *testing.T) {
	for d, want := range map[time.Duration]int{168 * time.Hour: 7, 7*24*time.Hour - time.Minute: 7, 14 * 24 * time.Hour: 14} {
		if got := TrialDays(d); got != want {
			t.Errorf("TrialDays(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestPagesKnowWhoIsOfferedPayments(t *testing.T) {
	t.Cleanup(func() { OfferBilling(nil, 0) })
	OfferBilling(func(id int64) bool { return id == 7 }, 7*24*time.Hour)

	page := func(u *User) (bool, int) {
		r := SetUserContext(httptest.NewRequest(http.MethodGet, "/", nil), u)
		p := NewPage(r, nil)
		return p.Billing, p.TrialDays
	}

	if billing, days := page(&User{ID: 7, Email: "t@b.c", Verified: true}); !billing || days != 7 {
		t.Errorf("the tester sees billing=%v, trial=%d days", billing, days)
	}
	if billing, _ := page(&User{ID: 8, Email: "o@b.c", Verified: true}); billing {
		t.Error("an account that is not offered payments sees them")
	}

	OfferBilling(nil, 0)
	if billing, _ := page(&User{ID: 7, Email: "t@b.c", Verified: true}); billing {
		t.Error("with billing off a page still sells Pro")
	}
}

func TestAFailedCancellationKeepsTheAccount(t *testing.T) {
	limiter := ratelimit.New(5, time.Minute)
	t.Cleanup(limiter.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	billing := &fakeBilling{cancelErr: errors.New("503")}
	h := &Handler{limiter: limiter, billing: billing, logger: logger}

	u := &User{ID: 42, Email: "a@b.c", Verified: true}
	if err := u.password.Set("correct horse battery"); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/account/delete", nil)
	sessions := session.NewStore(nil, time.Hour, false, "delete-test-secret", logger)
	sess, err := sessions.New(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal(err)
	}
	sess.UserID = &u.ID
	form := url.Values{"csrf_token": {sess.CSRF()}, "confirm": {"correct horse battery"}}
	r.Body = io.NopCloser(strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = SetUserContext(session.SetContext(clientip.SetContext(r, "203.0.113.9"), sess), u)

	w := httptest.NewRecorder()
	h.DeleteAccount(w, r)

	if w.Header().Get("Location") != "/account" || sess.Data[accountErrorMessage] != MsgCancelFailed {
		t.Errorf("location = %q, message = %q: an account was deleted while its subscription kept charging",
			w.Header().Get("Location"), sess.Data[accountErrorMessage])
	}
}
