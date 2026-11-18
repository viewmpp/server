package billing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/events"
	"server/internal/htmlutil"
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

type cancelCall struct {
	id  string
	now bool
}

type fakeAPI struct {
	portals   [][2]string
	cancelled []cancelCall
	err       error
}

func (f *fakeAPI) PortalURL(_ context.Context, customerID, subscriptionID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.portals = append(f.portals, [2]string{customerID, subscriptionID})
	return "https://sandbox-customer-portal.paddle.com/cpl_01?action=overview&token=t", nil
}

func (f *fakeAPI) Cancel(_ context.Context, id string, now bool) error {
	if f.err != nil {
		return f.err
	}
	f.cancelled = append(f.cancelled, cancelCall{id: id, now: now})
	return nil
}

type fakeCounts struct {
	names []events.Name
}

func (f *fakeCounts) Count(name events.Name) {
	f.names = append(f.names, name)
}

func testHandler(t *testing.T, store *fakeStore, api *fakeAPI, counts *fakeCounts) *Handler {
	t.Helper()

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(store, api, Settings{
		Secret:      testSecret,
		ClientToken: "test_token",
		Monthly:     testMonthly,
		Yearly:      testYearly,
		Sandbox:     true,
		TestUsers:   []int64{42},
		BaseURL:     "https://viewmpp.com/",
	}, templates, counts, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.now = func() time.Time { return testNow }
	return h
}

func deliver(h *Handler, body, signature string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/paddle", strings.NewReader(body))
	r.Header.Set(SignatureHeader, signature)
	w := httptest.NewRecorder()
	h.Webhook(w, r)
	return w
}

func TestASignedSubscriptionEventIsApplied(t *testing.T) {
	store, counts := &fakeStore{}, &fakeCounts{}
	h := testHandler(t, store, &fakeAPI{}, counts)

	body := subscriptionBody("subscription.created", "active", nil)
	w := deliver(h, body, sign(body))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(store.applied) != 1 || store.applied[0].UserID != 42 {
		t.Fatalf("applied = %+v", store.applied)
	}
	want := time.Date(2026, 12, 18, 10, 0, 0, 0, time.UTC).Add(RenewalGrace)
	if !store.access[0].Equal(want) {
		t.Errorf("access until %v, want %v", store.access[0], want)
	}
	if len(counts.names) != 1 || counts.names[0] != events.Billing("subscription.created") {
		t.Errorf("counted %v", counts.names)
	}
}

func TestAForgedOrReplayedEventChangesNothing(t *testing.T) {
	store := &fakeStore{}
	h := testHandler(t, store, &fakeAPI{}, &fakeCounts{})

	body := subscriptionBody("subscription.created", "active", nil)
	for name, signature := range map[string]string{
		"forged":   sign(strings.Replace(body, `"42"`, `"43"`, 1)),
		"replayed": signAt(body, testNow.Add(-time.Hour)),
	} {
		if w := deliver(h, body, signature); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d", name, w.Code)
		}
	}
	if len(store.applied) != 0 {
		t.Error("a forged or replayed webhook changed a plan")
	}
}

func TestEventsForAnotherProductOrSiteAreAcknowledgedAndIgnored(t *testing.T) {
	tests := map[string]map[string]string{
		"another price": {"price": `"pri_other"`},
		"another site":  {"site": `"http://localhost:4000"`},
	}
	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{}
			h := testHandler(t, store, &fakeAPI{}, &fakeCounts{})

			body := subscriptionBody("subscription.created", "active", fields)
			if w := deliver(h, body, sign(body)); w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 so Paddle stops retrying", w.Code)
			}
			if len(store.applied) != 0 {
				t.Error("an event for something else changed a plan")
			}
		})
	}
}

func TestTheYearlyPriceIsOursToo(t *testing.T) {
	store := &fakeStore{}
	h := testHandler(t, store, &fakeAPI{}, &fakeCounts{})

	body := subscriptionBody("subscription.created", "active", map[string]string{"price": `"` + testYearly + `"`})
	if w := deliver(h, body, sign(body)); w.Code != http.StatusOK || len(store.applied) != 1 {
		t.Errorf("status = %d, applied = %d", w.Code, len(store.applied))
	}
}

func TestFailuresAreAnsweredSoPaddleRetries(t *testing.T) {
	tests := []struct {
		name  string
		store *fakeStore
		body  string
		want  int
	}{
		{"an unknown status", &fakeStore{}, subscriptionBody("subscription.updated", "frozen", nil), http.StatusUnprocessableEntity},
		{"an event before the one that links it", &fakeStore{err: ErrUnknownSubscription}, subscriptionBody("subscription.updated", "active", nil), http.StatusConflict},
		{"the database is down", &fakeStore{err: errors.New("connection refused")}, subscriptionBody("subscription.updated", "active", nil), http.StatusInternalServerError},
		{"a payload we can't read", &fakeStore{}, `{"event_type":"subscription.updated","data":{"id":"sub_1","status":"active"}}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHandler(t, tt.store, &fakeAPI{}, &fakeCounts{})
			if w := deliver(h, tt.body, sign(tt.body)); w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestARepeatedEventIsAcknowledgedButNotCounted(t *testing.T) {
	counts := &fakeCounts{}
	h := testHandler(t, &fakeStore{outcome: Stale}, &fakeAPI{}, counts)

	body := subscriptionBody("subscription.updated", "active", nil)
	if w := deliver(h, body, sign(body)); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if len(counts.names) != 0 {
		t.Errorf("a duplicate was counted: %v", counts.names)
	}
}

func pageRequest(t *testing.T, target string, u *user.User) (*http.Request, *session.Session) {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, target, nil)
	sessions := session.NewStore(nil, time.Hour, false, "billing-test-secret", slog.New(slog.NewTextHandler(io.Discard, nil)))
	sess, err := sessions.New(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal(err)
	}
	if !u.IsAnonymous() {
		sess.UserID = &u.ID
	}
	return user.SetUserContext(session.SetContext(r, sess), u), sess
}

func verified() *user.User {
	return &user.User{ID: 42, Email: "buyer@example.com", Verified: true, Subscription: user.SubscriptionFree}
}

func pay(t *testing.T, h *Handler, target string, u *user.User) (*httptest.ResponseRecorder, *session.Session) {
	t.Helper()
	r, sess := pageRequest(t, target, u)
	w := httptest.NewRecorder()
	h.Pay(w, r)
	return w, sess
}

func TestThePayPageOpensPaddleForTheBuyersAccount(t *testing.T) {
	for plan, price := range map[string]string{PlanMonthly: testMonthly, PlanYearly: testYearly} {
		t.Run(plan, func(t *testing.T) {
			counts := &fakeCounts{}
			h := testHandler(t, &fakeStore{}, &fakeAPI{}, counts)

			w, _ := pay(t, h, "/billing/pay?plan="+plan, verified())
			body := w.Body.String()

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			for _, want := range []string{
				`data-price="` + price + `"`, `data-email="buyer@example.com"`, `data-user="42"`,
				`data-site="https://viewmpp.com"`, `data-sandbox="true"`, `data-token="test_token"`,
				`data-success="https://viewmpp.com/account?checkout=done"`,
				`src="https://cdn.paddle.com/paddle/v2/paddle.js"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the page is missing %s", want)
				}
			}
			if csp := w.Header().Get("Content-Security-Policy"); csp != PayPolicy {
				t.Errorf("csp = %q, want the policy that lets Paddle load", csp)
			}
			if len(counts.names) != 1 || counts.names[0] != events.CheckoutStart(plan) {
				t.Errorf("counted %v", counts.names)
			}
		})
	}
}

func TestThePayPageRefusesWhatItShouldNotSell(t *testing.T) {
	t.Run("signed out", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		w, _ := pay(t, h, "/billing/pay?plan=monthly", user.AnonymousUser)
		if w.Header().Get("Location") != "/signin" {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
	})

	t.Run("a plan we don't have", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		if w, _ := pay(t, h, "/billing/pay?plan=lifetime", verified()); w.Code != http.StatusBadRequest {
			t.Errorf("status = %d", w.Code)
		}
	})

	t.Run("an unconfirmed email", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		u := verified()
		u.Verified = false
		w, sess := pay(t, h, "/billing/pay?plan=monthly", u)
		if w.Header().Get("Location") != "/account" || sess.Data["flash"] != user.MsgSubscribeNeedsEmail {
			t.Errorf("location = %q, flash = %q", w.Header().Get("Location"), sess.Data["flash"])
		}
	})

	t.Run("not a tester in the sandbox", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		u := verified()
		u.ID = 7
		if w, _ := pay(t, h, "/billing/pay?plan=monthly", u); w.Code != http.StatusNotFound {
			t.Errorf("status = %d: anyone could get Pro with a test card", w.Code)
		}
	})

	t.Run("a second subscription", func(t *testing.T) {
		live := &Subscription{UserID: 42, SubscriptionID: "sub_01", Status: StatusActive, AccessUntil: testNow.Add(time.Hour), TestMode: true}
		h := testHandler(t, &fakeStore{found: live}, &fakeAPI{}, &fakeCounts{})
		if w, _ := pay(t, h, "/billing/pay?plan=yearly", verified()); w.Header().Get("Location") != "/billing/portal" {
			t.Errorf("location = %q: a subscriber was sent to pay twice", w.Header().Get("Location"))
		}
	})
}

func TestFreeProIsNotChargedForWhileItHasMoreThanAWeekLeft(t *testing.T) {
	until := func(d time.Duration) *time.Time {
		at := testNow.Add(d)
		return &at
	}

	t.Run("early access for three months", func(t *testing.T) {
		counts := &fakeCounts{}
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, counts)
		u := verified()
		u.Subscription, u.SubscriptionUntil = user.SubscriptionPro, until(90*24*time.Hour)

		w, sess := pay(t, h, "/billing/pay?plan=monthly", u)
		if w.Header().Get("Location") != "/account" || sess.Data["flash"] != user.MsgBuyLater {
			t.Errorf("location = %q, flash = %q: a buyer was charged for months Pro is already free", w.Header().Get("Location"), sess.Data["flash"])
		}
		if len(counts.names) != 0 {
			t.Errorf("counted %v", counts.names)
		}
	})

	t.Run("a lifetime grant", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		u := verified()
		u.Subscription = user.SubscriptionPro

		if w, _ := pay(t, h, "/billing/pay?plan=monthly", u); w.Header().Get("Location") != "/account" {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
	})

	t.Run("the last days of a trial", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		u := verified()
		u.Subscription, u.SubscriptionUntil = user.SubscriptionPro, until(3*24*time.Hour)

		if w, _ := pay(t, h, "/billing/pay?plan=monthly", u); w.Code != http.StatusOK {
			t.Errorf("status = %d, location = %q: a trial user could not subscribe", w.Code, w.Header().Get("Location"))
		}
	})
}

func TestAPaddleLinkWithATransactionOpensWithoutSigningIn(t *testing.T) {
	counts := &fakeCounts{}
	h := testHandler(t, &fakeStore{}, &fakeAPI{}, counts)

	w, _ := pay(t, h, "/billing/pay?_ptxn=txn_01", user.AnonymousUser)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `data-transaction="txn_01"`) {
		t.Errorf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `data-email="buyer`) || len(counts.names) != 0 {
		t.Error("a Paddle transaction link carried account details or counted as a new checkout")
	}
}

func TestAnExpiredSubscriberCanSubscribeAgain(t *testing.T) {
	over := &Subscription{UserID: 42, SubscriptionID: "sub_01", Status: StatusExpired, AccessUntil: testNow.Add(-time.Hour), TestMode: true}
	h := testHandler(t, &fakeStore{found: over}, &fakeAPI{}, &fakeCounts{})

	if w, _ := pay(t, h, "/billing/pay?plan=monthly", verified()); w.Code != http.StatusOK {
		t.Errorf("status = %d, location = %q", w.Code, w.Header().Get("Location"))
	}
}

func TestThePortalOpensForASubscriberOnly(t *testing.T) {
	sub := &Subscription{SubscriptionID: "sub_01", CustomerID: "ctm_01", Status: StatusActive, AccessUntil: testNow.Add(time.Hour), TestMode: true}

	t.Run("subscriber", func(t *testing.T) {
		api := &fakeAPI{}
		h := testHandler(t, &fakeStore{found: sub}, api, &fakeCounts{})
		r, _ := pageRequest(t, "/billing/portal", verified())
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if !strings.HasPrefix(w.Header().Get("Location"), "https://sandbox-customer-portal.paddle.com/") {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
		if len(api.portals) != 1 || api.portals[0] != [2]string{"ctm_01", "sub_01"} {
			t.Errorf("portal calls = %v", api.portals)
		}
	})

	t.Run("never subscribed", func(t *testing.T) {
		h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})
		r, _ := pageRequest(t, "/billing/portal", verified())
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if w.Header().Get("Location") != "/pricing" {
			t.Errorf("location = %q", w.Header().Get("Location"))
		}
	})

	t.Run("paddle is down", func(t *testing.T) {
		h := testHandler(t, &fakeStore{found: sub}, &fakeAPI{err: errors.New("503")}, &fakeCounts{})
		r, sess := pageRequest(t, "/billing/portal", verified())
		w := httptest.NewRecorder()
		h.Portal(w, r)
		if w.Header().Get("Location") != "/account" || sess.Data["flash"] != MsgPortalDown {
			t.Errorf("location = %q, flash = %q", w.Header().Get("Location"), sess.Data["flash"])
		}
	})
}

func TestInTheSandboxOnlyTestersArePaymentsOffered(t *testing.T) {
	h := testHandler(t, &fakeStore{}, &fakeAPI{}, &fakeCounts{})

	if h.Offered(7) || h.Offered(0) {
		t.Error("a visitor who is not a tester was offered sandbox payments")
	}
	if !h.Offered(42) {
		t.Error("a tester was not offered payments")
	}

	h.settings.Sandbox = false
	if !h.Offered(7) || !h.Offered(0) {
		t.Error("live payments are not offered to everyone")
	}
}

func TestASubscriptionFromTheOtherEnvironmentIsNotOurs(t *testing.T) {
	sandboxSub := &Subscription{SubscriptionID: "sub_01", CustomerID: "ctm_01", Status: StatusActive, AccessUntil: testNow.Add(time.Hour), TestMode: true}
	api := &fakeAPI{}
	h := testHandler(t, &fakeStore{found: sandboxSub}, api, &fakeCounts{})
	h.settings.Sandbox = false

	plan, err := h.Plan(context.Background(), 42)
	if err != nil || plan != nil {
		t.Errorf("plan = %+v, err = %v: a sandbox subscription showed as paid after launch", plan, err)
	}
	if err = h.Cancel(context.Background(), 42); err != nil || len(api.cancelled) != 0 {
		t.Errorf("cancelled %v, err = %v: a live key was sent a sandbox subscription", api.cancelled, err)
	}
	if w, _ := pay(t, h, "/billing/pay?plan=monthly", verified()); w.Code != http.StatusOK {
		t.Errorf("a leftover sandbox subscription blocked a real purchase: status %d, location %q", w.Code, w.Header().Get("Location"))
	}
}

func TestThePlanShownIsTheLiveOne(t *testing.T) {
	renews := testNow.Add(30 * 24 * time.Hour)
	tests := []struct {
		name string
		sub  *Subscription
		want *user.PaidPlan
	}{
		{"monthly", &Subscription{Status: StatusActive, VariantID: testMonthly, AccessUntil: renews, PeriodEnd: &renews, TestMode: true},
			&user.PaidPlan{Status: StatusActive, PeriodEnd: &renews}},
		{"yearly, cancelling", &Subscription{Status: StatusCancelled, VariantID: testYearly, AccessUntil: renews, PeriodEnd: &renews, TestMode: true},
			&user.PaidPlan{Status: StatusCancelled, Yearly: true, PeriodEnd: &renews}},
		{"over", &Subscription{Status: StatusExpired, VariantID: testMonthly, AccessUntil: testNow, TestMode: true}, nil},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := testHandler(t, &fakeStore{found: tt.sub}, &fakeAPI{}, &fakeCounts{})
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
	for status, want := range map[string]*cancelCall{
		StatusActive:    {id: "sub_01"},
		StatusTrial:     {id: "sub_01"},
		StatusPastDue:   {id: "sub_01"},
		StatusPaused:    {id: "sub_01", now: true},
		StatusCancelled: nil,
		StatusExpired:   nil,
	} {
		t.Run(status, func(t *testing.T) {
			api := &fakeAPI{}
			sub := &Subscription{SubscriptionID: "sub_01", Status: status, AccessUntil: testNow.Add(time.Hour), TestMode: true}
			h := testHandler(t, &fakeStore{found: sub}, api, &fakeCounts{})

			if err := h.Cancel(context.Background(), 42); err != nil {
				t.Fatal(err)
			}
			switch {
			case want == nil && len(api.cancelled) != 0:
				t.Errorf("cancelled %v, want nothing", api.cancelled)
			case want != nil && (len(api.cancelled) != 1 || api.cancelled[0] != *want):
				t.Errorf("cancelled %v, want %v", api.cancelled, *want)
			}
		})
	}

	t.Run("paddle is down", func(t *testing.T) {
		sub := &Subscription{SubscriptionID: "sub_01", Status: StatusActive, AccessUntil: testNow.Add(time.Hour), TestMode: true}
		h := testHandler(t, &fakeStore{found: sub}, &fakeAPI{err: errors.New("503")}, &fakeCounts{})
		if err := h.Cancel(context.Background(), 42); err == nil {
			t.Error("a failed cancellation was reported as done; the account would be deleted and still charged")
		}
	})
}
