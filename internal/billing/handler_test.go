package billing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"server/internal/events"
	"server/internal/session"
	"server/internal/user"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	applied []Event
	access  []time.Time
	outcome Outcome
	err     error
	found   *Subscription
}

func (f *fakeStore) Apply(_ context.Context, e Event, access time.Time) (Outcome, int64, error) {
	if f.err != nil {
		return "", 0, f.err
	}
	f.applied = append(f.applied, e)
	f.access = append(f.access, access)
	if f.outcome == "" {
		return Applied, e.UserID, nil
	}
	return f.outcome, e.UserID, nil
}

func (f *fakeStore) Find(context.Context, int64) (*Subscription, error) {
	return f.found, nil
}

type fakeAPI struct {
	checkouts []Checkout
	cancelled []string
	err       error
}

func (f *fakeAPI) Cancel(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.cancelled = append(f.cancelled, id)
	return nil
}

func (f *fakeAPI) CreateCheckout(_ context.Context, ch Checkout) (string, error) {
	f.checkouts = append(f.checkouts, ch)
	if f.err != nil {
		return "", f.err
	}
	return "https://viewmpp.lemonsqueezy.com/checkout/custom/abc", nil
}

func (f *fakeAPI) PortalURL(context.Context, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "https://viewmpp.lemonsqueezy.com/billing?expires=1", nil
}

type fakeCounts struct {
	names []events.Name
}

func (f *fakeCounts) Count(name events.Name) {
	f.names = append(f.names, name)
}

var testNow = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func testHandler(store *fakeStore, api *fakeAPI, counts *fakeCounts) *Handler {
	h := NewHandler(store, api, Settings{
		Secret:    testSecret,
		StoreID:   "513638",
		Monthly:   "2370696",
		Yearly:    "2370717",
		TestMode:  true,
		TestUsers: []int64{42},
		BaseURL:   "https://viewmpp.com",
	}, counts, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.now = func() time.Time { return testNow }
	return h
}

func deliver(h *Handler, body, signature string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/lemonsqueezy", strings.NewReader(body))
	r.Header.Set(SignatureHeader, signature)
	w := httptest.NewRecorder()
	h.Webhook(w, r)
	return w
}

func TestASignedSubscriptionEventIsApplied(t *testing.T) {
	store, counts := &fakeStore{}, &fakeCounts{}
	h := testHandler(store, &fakeAPI{}, counts)

	body := subscriptionBody("subscription_created", "active", nil)
	w := deliver(h, body, sign(body))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(store.applied) != 1 || store.applied[0].UserID != 42 {
		t.Fatalf("applied = %+v", store.applied)
	}
	want := time.Date(2026, 12, 13, 10, 0, 0, 0, time.UTC).Add(RenewalGrace)
	if !store.access[0].Equal(want) {
		t.Errorf("access until %v, want %v", store.access[0], want)
	}
	if len(counts.names) != 1 || counts.names[0] != events.Billing("subscription_created") {
		t.Errorf("counted %v", counts.names)
	}
}

func TestAForgedEventChangesNothing(t *testing.T) {
	store := &fakeStore{}
	h := testHandler(store, &fakeAPI{}, &fakeCounts{})

	body := subscriptionBody("subscription_created", "active", nil)
	w := deliver(h, body, sign(strings.Replace(body, "42", "43", 1)))

	if w.Code != http.StatusUnauthorized || len(store.applied) != 0 {
		t.Errorf("status = %d, applied = %d: a forged webhook got through", w.Code, len(store.applied))
	}
}

func TestEventsForAnotherProductAreAcknowledgedAndIgnored(t *testing.T) {
	tests := map[string]map[string]string{
		"another store":      {"store_id": "1"},
		"another variant":    {"variant_id": "999"},
		"live while we test": {"test_mode": "false"},
	}
	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			h := testHandler(store, &fakeAPI{}, &fakeCounts{})

			body := subscriptionBody("subscription_created", "active", fields)
			w := deliver(h, body, sign(body))

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 so Lemon Squeezy stops retrying", w.Code)
			}
			if len(store.applied) != 0 {
				t.Error("an event for something we don't sell changed a plan")
			}
		})
	}
}

func TestTheYearlyVariantIsOursToo(t *testing.T) {
	store := &fakeStore{}
	h := testHandler(store, &fakeAPI{}, &fakeCounts{})

	body := subscriptionBody("subscription_created", "active", map[string]string{"variant_id": "2370717"})
	if w := deliver(h, body, sign(body)); w.Code != http.StatusOK || len(store.applied) != 1 {
		t.Errorf("status = %d, applied = %d", w.Code, len(store.applied))
	}
}

func TestFailuresAreAnsweredSoLemonSqueezyRetries(t *testing.T) {
	tests := []struct {
		name  string
		store *fakeStore
		body  string
		want  int
	}{
		{"an unknown status", &fakeStore{}, subscriptionBody("subscription_updated", "frozen", nil), http.StatusUnprocessableEntity},
		{"a renewal that arrived before its first event", &fakeStore{err: ErrUnknownSubscription}, subscriptionBody("subscription_updated", "active", nil), http.StatusConflict},
		{"the database is down", &fakeStore{err: errors.New("connection refused")}, subscriptionBody("subscription_updated", "active", nil), http.StatusInternalServerError},
		{"a payload we can't read", &fakeStore{}, `{"data":{"type":"subscriptions","id":"1","attributes":{"status":"active"}}}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHandler(tt.store, &fakeAPI{}, &fakeCounts{})
			if w := deliver(h, tt.body, sign(tt.body)); w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestARepeatedEventIsAcknowledgedButNotCounted(t *testing.T) {
	counts := &fakeCounts{}
	h := testHandler(&fakeStore{outcome: Stale}, &fakeAPI{}, counts)

	body := subscriptionBody("subscription_updated", "active", nil)
	if w := deliver(h, body, sign(body)); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if len(counts.names) != 0 {
		t.Errorf("a duplicate was counted: %v", counts.names)
	}
}

func formRequest(t *testing.T, target string, form url.Values, u *user.User) (*http.Request, *session.Session) {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, target, nil)
	sessions := session.NewStore(nil, time.Hour, false, "billing-test-secret", slog.New(slog.NewTextHandler(io.Discard, nil)))
	sess, err := sessions.New(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal(err)
	}
	sess.UserID = &u.ID

	if form == nil {
		form = url.Values{}
	}
	form.Set(session.CSRFField, sess.CSRF())
	r.Body = io.NopCloser(strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", "https://viewmpp.com/pricing")

	return user.SetUserContext(session.SetContext(r, sess), u), sess
}

func verified() *user.User {
	return &user.User{ID: 42, Email: "buyer@example.com", Verified: true, Subscription: user.SubscriptionFree}
}

func TestCheckoutSendsTheBuyerToLemonSqueezyWithTheirAccount(t *testing.T) {
	for plan, variant := range map[string]string{PlanMonthly: "2370696", PlanYearly: "2370717"} {
		t.Run(plan, func(t *testing.T) {
			api, counts := &fakeAPI{}, &fakeCounts{}
			h := testHandler(&fakeStore{}, api, counts)

			r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {plan}}, verified())
			w := httptest.NewRecorder()
			h.Checkout(w, r)

			if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "https://viewmpp.lemonsqueezy.com/checkout/custom/abc" {
				t.Fatalf("status = %d, location = %q", w.Code, w.Header().Get("Location"))
			}
			want := Checkout{VariantID: variant, UserID: 42, Email: "buyer@example.com", Site: "https://viewmpp.com", RedirectTo: "https://viewmpp.com/account?checkout=done"}
			if len(api.checkouts) != 1 || api.checkouts[0] != want {
				t.Errorf("checkouts = %+v, want %+v", api.checkouts, want)
			}
			if len(counts.names) != 1 || counts.names[0] != events.CheckoutStart(plan) {
				t.Errorf("counted %v", counts.names)
			}
		})
	}
}

func TestCheckoutRefusesWhatItShouldNotSell(t *testing.T) {
	t.Run("a plan we don't have", func(t *testing.T) {
		api := &fakeAPI{}
		h := testHandler(&fakeStore{}, api, &fakeCounts{})
		r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {"lifetime"}}, verified())
		w := httptest.NewRecorder()
		h.Checkout(w, r)
		if w.Code != http.StatusBadRequest || len(api.checkouts) != 0 {
			t.Errorf("status = %d, checkouts = %d", w.Code, len(api.checkouts))
		}
	})

	t.Run("an unconfirmed email", func(t *testing.T) {
		api := &fakeAPI{}
		h := testHandler(&fakeStore{}, api, &fakeCounts{})
		u := verified()
		u.Verified = false
		r, sess := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, u)
		w := httptest.NewRecorder()
		h.Checkout(w, r)
		if w.Header().Get("Location") != "/pricing" || len(api.checkouts) != 0 || sess.Data["flash"] != user.MsgSubscribeNeedsEmail {
			t.Errorf("location = %q, checkouts = %d, flash = %q", w.Header().Get("Location"), len(api.checkouts), sess.Data["flash"])
		}
	})

	t.Run("a second subscription", func(t *testing.T) {
		api := &fakeAPI{}
		live := &Subscription{UserID: 42, SubscriptionID: "901", Status: "active", AccessUntil: testNow.Add(time.Hour), TestMode: true}
		h := testHandler(&fakeStore{found: live}, api, &fakeCounts{})
		r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanYearly}}, verified())
		w := httptest.NewRecorder()
		h.Checkout(w, r)
		if w.Header().Get("Location") != "/billing/portal" || len(api.checkouts) != 0 {
			t.Errorf("location = %q, checkouts = %d: a subscriber was charged twice", w.Header().Get("Location"), len(api.checkouts))
		}
	})

	t.Run("no csrf token", func(t *testing.T) {
		api := &fakeAPI{}
		h := testHandler(&fakeStore{}, api, &fakeCounts{})
		r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, verified())
		r.PostForm = nil
		r.Form = nil
		r.Body = io.NopCloser(strings.NewReader("plan=monthly"))
		w := httptest.NewRecorder()
		h.Checkout(w, r)
		if w.Code != http.StatusBadRequest || len(api.checkouts) != 0 {
			t.Errorf("status = %d, checkouts = %d", w.Code, len(api.checkouts))
		}
	})
}

func TestAnExpiredSubscriberCanSubscribeAgain(t *testing.T) {
	api := &fakeAPI{}
	over := &Subscription{UserID: 42, SubscriptionID: "901", Status: "expired", AccessUntil: testNow.Add(-time.Hour), TestMode: true}
	h := testHandler(&fakeStore{found: over}, api, &fakeCounts{})

	r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, verified())
	w := httptest.NewRecorder()
	h.Checkout(w, r)

	if len(api.checkouts) != 1 {
		t.Errorf("checkouts = %d, location = %q", len(api.checkouts), w.Header().Get("Location"))
	}
}

func TestCheckoutTellsTheBuyerWhenPaymentsAreDown(t *testing.T) {
	h := testHandler(&fakeStore{}, &fakeAPI{err: errors.New("timeout")}, &fakeCounts{})

	r, sess := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, verified())
	w := httptest.NewRecorder()
	h.Checkout(w, r)

	if w.Header().Get("Location") != "/pricing" || sess.Data["flash"] != MsgPaymentsDown {
		t.Errorf("location = %q, flash = %q", w.Header().Get("Location"), sess.Data["flash"])
	}
}

func portalRequest(t *testing.T) (*http.Request, *session.Session) {
	t.Helper()
	r, sess := formRequest(t, "/billing/portal", nil, verified())
	r.Method = http.MethodGet
	return r, sess
}

func TestThePortalOpensForASubscriberOnly(t *testing.T) {
	t.Run("subscriber", func(t *testing.T) {
		h := testHandler(&fakeStore{found: &Subscription{SubscriptionID: "901", TestMode: true}}, &fakeAPI{}, &fakeCounts{})
		r, _ := portalRequest(t)
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if w.Header().Get("Location") != "https://viewmpp.lemonsqueezy.com/billing?expires=1" {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
	})

	t.Run("never subscribed", func(t *testing.T) {
		h := testHandler(&fakeStore{}, &fakeAPI{}, &fakeCounts{})
		r, _ := portalRequest(t)
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if w.Header().Get("Location") != "/pricing" {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
	})

	t.Run("lemon squeezy is down", func(t *testing.T) {
		h := testHandler(&fakeStore{found: &Subscription{SubscriptionID: "901", TestMode: true}}, &fakeAPI{err: errors.New("503")}, &fakeCounts{})
		r, sess := portalRequest(t)
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if w.Header().Get("Location") != "/account" || sess.Data["flash"] != MsgPortalDown {
			t.Errorf("location = %q, flash = %q", w.Header().Get("Location"), sess.Data["flash"])
		}
	})
}

func TestInTestModeOnlyTestersArePaymentsOffered(t *testing.T) {
	api := &fakeAPI{}
	h := testHandler(&fakeStore{}, api, &fakeCounts{})
	h.testers = map[int64]bool{7: true}

	if h.Offered(42) || h.Offered(0) {
		t.Error("a visitor who is not a tester was offered test payments; anyone could get Pro with a test card")
	}
	if !h.Offered(7) {
		t.Error("a tester was not offered payments")
	}

	r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, verified())
	w := httptest.NewRecorder()
	h.Checkout(w, r)
	if w.Code != http.StatusNotFound || len(api.checkouts) != 0 {
		t.Errorf("status = %d, checkouts = %d: a non-tester reached a test checkout", w.Code, len(api.checkouts))
	}

	h.settings.TestMode = false
	if !h.Offered(42) || !h.Offered(0) {
		t.Error("live payments are not offered to everyone")
	}
}

func TestASubscriptionFromTheOtherModeIsNotOurs(t *testing.T) {
	test := &Subscription{SubscriptionID: "901", Status: "active", AccessUntil: testNow.Add(time.Hour), TestMode: true}
	api := &fakeAPI{}
	h := testHandler(&fakeStore{found: test}, api, &fakeCounts{})
	h.settings.TestMode = false

	plan, err := h.Plan(context.Background(), 42)
	if err != nil || plan != nil {
		t.Errorf("plan = %+v, err = %v: a test subscription showed as paid after launch", plan, err)
	}
	if err = h.Cancel(context.Background(), 42); err != nil || len(api.cancelled) != 0 {
		t.Errorf("cancelled %v, err = %v: a live key was sent a test subscription", api.cancelled, err)
	}

	r, _ := formRequest(t, "/billing/checkout", url.Values{"plan": {PlanMonthly}}, verified())
	w := httptest.NewRecorder()
	h.Checkout(w, r)
	if len(api.checkouts) != 1 {
		t.Errorf("a leftover test subscription blocked a real purchase: location = %q", w.Header().Get("Location"))
	}
}

func TestThePlanShownIsTheLiveOne(t *testing.T) {
	renews := testNow.Add(30 * 24 * time.Hour)
	tests := []struct {
		name string
		sub  *Subscription
		want *user.PaidPlan
	}{
		{"monthly", &Subscription{Status: "active", VariantID: "2370696", AccessUntil: renews, PeriodEnd: &renews, TestMode: true},
			&user.PaidPlan{Status: "active", PeriodEnd: &renews}},
		{"yearly", &Subscription{Status: "cancelled", VariantID: "2370717", AccessUntil: renews, PeriodEnd: &renews, TestMode: true},
			&user.PaidPlan{Status: "cancelled", Yearly: true, PeriodEnd: &renews}},
		{"over", &Subscription{Status: "expired", VariantID: "2370696", AccessUntil: testNow, TestMode: true}, nil},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHandler(&fakeStore{found: tt.sub}, &fakeAPI{}, &fakeCounts{})
			got, err := h.Plan(context.Background(), 42)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (tt.want == nil) || got != nil && (got.Status != tt.want.Status || got.Yearly != tt.want.Yearly || !got.PeriodEnd.Equal(*tt.want.PeriodEnd)) {
				t.Errorf("plan = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDeletingAnAccountCancelsOnlyWhatStillCharges(t *testing.T) {
	for status, wantCancel := range map[string]bool{
		"active": true, "on_trial": true, "past_due": true, "paused": true, "unpaid": true,
		"cancelled": false, "expired": false,
	} {
		t.Run(status, func(t *testing.T) {
			api := &fakeAPI{}
			sub := &Subscription{SubscriptionID: "901", Status: status, AccessUntil: testNow.Add(time.Hour), TestMode: true}
			h := testHandler(&fakeStore{found: sub}, api, &fakeCounts{})

			if err := h.Cancel(context.Background(), 42); err != nil {
				t.Fatal(err)
			}
			if got := len(api.cancelled) == 1; got != wantCancel {
				t.Errorf("cancelled = %v, want %v", api.cancelled, wantCancel)
			}
		})
	}

	t.Run("lemon squeezy is down", func(t *testing.T) {
		sub := &Subscription{SubscriptionID: "901", Status: "active", AccessUntil: testNow.Add(time.Hour), TestMode: true}
		h := testHandler(&fakeStore{found: sub}, &fakeAPI{err: errors.New("503")}, &fakeCounts{})
		if err := h.Cancel(context.Background(), 42); err == nil {
			t.Error("a failed cancellation was reported as done; the account would be deleted and still charged")
		}
	})
}

func TestACheckoutMadeOnAnotherSiteDoesNotGrantProHere(t *testing.T) {
	store := &fakeStore{}
	h := testHandler(store, &fakeAPI{}, &fakeCounts{})

	local := strings.Replace(subscriptionBody("subscription_created", "active", nil),
		`"custom_data": {"user_id": "42"}`, `"custom_data": {"user_id": "42", "site": "http://localhost:4000"}`, 1)
	if w := deliver(h, local, sign(local)); w.Code != http.StatusOK || len(store.applied) != 0 {
		t.Errorf("status = %d, applied = %d: a test purchase on a laptop gave Pro to whoever has the same id in production",
			w.Code, len(store.applied))
	}

	ours := strings.Replace(subscriptionBody("subscription_created", "active", nil),
		`"custom_data": {"user_id": "42"}`, `"custom_data": {"user_id": "42", "site": "https://viewmpp.com"}`, 1)
	if w := deliver(h, ours, sign(ours)); w.Code != http.StatusOK || len(store.applied) != 1 {
		t.Errorf("status = %d, applied = %d: our own checkout was ignored", w.Code, len(store.applied))
	}
}
