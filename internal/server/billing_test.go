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
	s.billingHandler = billing.NewHandler(nil, nil, billing.Settings{Secret: "pdl_ntfset_test"}, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s
}

func TestTheWebhookIsReachedWithoutASessionOrOurOrigin(t *testing.T) {
	s := billingServer(t)

	r := httptest.NewRequest(http.MethodPost, "/webhooks/paddle", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://vendors.paddle.com")
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
	routes := map[string]string{
		"POST /webhooks/paddle": "top",
		"GET /billing/pay":      "mux",
		"GET /billing/portal":   "mux",
	}

	off, on := originServer(t), billingServer(t)
	for route, where := range routes {
		method, path, _ := strings.Cut(route, " ")
		r := httptest.NewRequest(method, path, nil)

		pattern := func(s *Server) string {
			if where == "top" {
				_, p := s.top().Handler(r)
				return p
			}
			_, p := s.mux().Handler(r)
			return p
		}

		if p := pattern(off); p == route {
			t.Errorf("%s is routed with billing off", route)
		}
		if p := pattern(on); p != route {
			t.Errorf("%s is routed to %q with billing on", route, p)
		}
	}
}

func TestOnlyThePayPageMayLoadPaddle(t *testing.T) {
	if strings.Contains(policy, "paddle") {
		t.Errorf("policy = %q: every page would let Paddle scripts run", policy)
	}
	for _, want := range []string{"script-src 'self' 'unsafe-inline' https://cdn.paddle.com", "frame-src https://*.paddle.com", "frame-ancestors 'none'"} {
		if !strings.Contains(billing.PayPolicy, want) {
			t.Errorf("pay policy is missing %q", want)
		}
	}
}
