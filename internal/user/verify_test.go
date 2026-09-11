package user

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"server/internal/htmlutil"
	"server/internal/ratelimit"
	"server/internal/session"
	"strings"
	"testing"
	"time"
)

func verificationRequest(t *testing.T, method, target, code string) (*Handler, *http.Request, *User, *session.Session) {
	t.Helper()

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		t.Fatal(err)
	}
	limiter := ratelimit.New(1, time.Minute)
	t.Cleanup(limiter.Close)
	h := &Handler{
		templates: templates,
		limiter:   limiter,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	r := httptest.NewRequest(method, target, nil)
	store := session.NewStore(nil, time.Hour, false, "verification-test-secret", h.logger)
	sess, err := store.New(httptest.NewRecorder(), r)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{ID: 42, Email: "unverified@example.com", Subscription: SubscriptionFree}
	sess.UserID = &u.ID
	sess.Data["pending_email"] = u.Email

	form := url.Values{"csrf_token": {sess.CSRF()}, "code": {code}}
	r.Body = io.NopCloser(strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = SetUserContext(session.SetContext(r, sess), u)
	return h, r, u, sess
}

func TestVerificationPageReloadDoesNotConfirmAccount(t *testing.T) {
	h, r, u, sess := verificationRequest(t, http.MethodGet, "/verify?code=ABCDEFGH", "ABCDEFGH")

	for range 3 {
		w := httptest.NewRecorder()
		h.VerifyPage(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200", w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("verification page can be cached")
		}
		if u.Verified || sess.Get("pending_email") != u.Email || sess.Get("flash") == MsgEmailConfirmed {
			t.Fatal("GET changed verification state")
		}
		if strings.Contains(w.Body.String(), MsgEmailConfirmed) || w.Header().Get("Location") != "" {
			t.Fatal("GET reported successful verification")
		}
	}
}

func TestVerificationRejectsMissingCodeWithValidCSRF(t *testing.T) {
	for _, test := range []struct{ name, code string }{{"empty", ""}, {"whitespace", " \t\r\n"}} {
		t.Run(test.name, func(t *testing.T) {
			h, r, u, sess := verificationRequest(t, http.MethodPost, "/verify?code=ABCDEFGH", test.code)
			w := httptest.NewRecorder()
			h.Verify(w, r)
			if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), MsgCodeRequired) {
				t.Fatalf("empty POST status = %d, want 422 with missing-code error", w.Code)
			}
			if u.Verified || sess.Get("pending_email") != u.Email || w.Header().Get("Location") != "" {
				t.Fatal("empty POST confirmed the account or redirected")
			}
		})
	}
}

func TestVerificationThrottleDoesNotConfirmAccount(t *testing.T) {
	h, r, u, sess := verificationRequest(t, http.MethodPost, "/verify", "ABCDEFGH")
	h.limiter.Take(VisitorKey(r, "verify:"))
	w := httptest.NewRecorder()
	h.Verify(w, r)
	if w.Code != http.StatusTooManyRequests || !strings.Contains(w.Body.String(), MsgTooManyTries) {
		t.Fatalf("throttled POST status = %d, want 429 with throttle error", w.Code)
	}
	if u.Verified || sess.Get("pending_email") != u.Email || w.Header().Get("Location") != "" {
		t.Fatal("throttled POST confirmed the account or redirected")
	}
}
