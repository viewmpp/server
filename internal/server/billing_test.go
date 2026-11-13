package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"server/internal/billing"
	"strings"
	"testing"
)

func billingServer(t *testing.T) *Server {
	t.Helper()

	s := originServer(t)
	s.billingHandler = billing.NewHandler(nil, nil, billing.Settings{Secret: "secret"}, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s
}

func TestTheWebhookIsReachedWithoutASessionOrOurOrigin(t *testing.T) {
	s := billingServer(t)

	r := httptest.NewRequest(http.MethodPost, "/webhooks/lemonsqueezy", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://app.lemonsqueezy.com")
	w := httptest.NewRecorder()

	s.top().ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 from the signature check: the webhook never reached its handler", w.Code)
	}
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("the webhook set cookies %v; every delivery would leave a session row behind", cookies)
	}
}

func TestBillingRoutesExistOnlyWhenBillingIsOn(t *testing.T) {
	s := originServer(t)

	for _, path := range []string{"/webhooks/lemonsqueezy", "/billing/checkout", "/billing/portal"} {
		_, pattern := s.top().Handler(httptest.NewRequest(http.MethodPost, path, nil))
		if pattern != "/" {
			t.Errorf("%s is routed to %q with billing off", path, pattern)
		}
	}

	on := billingServer(t)
	if _, pattern := on.top().Handler(httptest.NewRequest(http.MethodPost, "/webhooks/lemonsqueezy", nil)); pattern != "POST /webhooks/lemonsqueezy" {
		t.Errorf("webhook pattern = %q with billing on", pattern)
	}
}

func TestTheCheckoutFormMayLeaveForLemonSqueezy(t *testing.T) {
	if !strings.Contains(policy, "form-action 'self' https://*.lemonsqueezy.com") {
		t.Errorf("policy = %q: the browser would block the redirect from our checkout form to the payment page", policy)
	}
}
