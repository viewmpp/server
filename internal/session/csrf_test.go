package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestStore(secure bool) *Store {
	return &Store{lifetime: 12 * time.Hour, secure: secure}
}

func renderForm(t *testing.T, store *Store, r *http.Request) (string, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w, csrf: store.readCSRF(r)}

	return sess.CSRF(), w
}

func TestATokenIsMintedAndHandedOutAsACookie(t *testing.T) {
	store := newTestStore(false)

	token, w := renderForm(t, store, httptest.NewRequest(http.MethodGet, "/signin", nil))

	if token == "" {
		t.Fatal("no token was minted for a form")
	}

	cookie := w.Result().Cookies()[0]

	if cookie.Value != token {
		t.Errorf("cookie carries %q but the form carries %q: double submit compares the two", cookie.Value, token)
	}

	if !cookie.HttpOnly {
		t.Error("the csrf cookie is readable from javascript")
	}
}

func TestRenderingAFormWritesNothingToTheSession(t *testing.T) {
	store := newTestStore(false)

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w}

	sess.CSRF()

	if len(sess.Data) != 0 {
		t.Fatalf("session data holds %v: a form page must not create server state, that is what put a row in the table for every bot", sess.Data)
	}
}

func TestTheTokenSurvivesAcrossRequests(t *testing.T) {
	store := newTestStore(false)

	first, w := renderForm(t, store, httptest.NewRequest(http.MethodGet, "/signin", nil))

	next := httptest.NewRequest(http.MethodGet, "/signin", nil)
	for _, c := range w.Result().Cookies() {
		next.AddCookie(c)
	}

	second, _ := renderForm(t, store, next)

	if first != second {
		t.Error("a second page mints a new token: every reload would invalidate the form left open in another tab")
	}
}

func post(t *testing.T, store *Store, cookieValue, formValue string) bool {
	t.Helper()

	body := strings.NewReader(CSRFField + "=" + formValue)
	r := httptest.NewRequest(http.MethodPost, "/signin", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if cookieValue != "" {
		r.AddCookie(&http.Cookie{Name: store.csrfName(), Value: cookieValue})
	}

	sess := &Session{Data: map[string]string{}, store: store, csrf: store.readCSRF(r)}

	return VerifyCSRF(sess, r)
}

func TestVerification(t *testing.T) {
	store := newTestStore(false)

	tests := []struct {
		name   string
		cookie string
		form   string
		want   bool
	}{
		{"matching pair", "abc", "abc", true},
		{"form field forged", "abc", "xyz", false},
		{"no cookie", "", "abc", false},
		{"no form field", "abc", "", false},
		{"neither", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := post(t, store, tc.cookie, tc.form); got != tc.want {
				t.Errorf("accepted = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTheCookieIsLockedToTheHostInProduction(t *testing.T) {
	if got := newTestStore(true).csrfName(); got != "__Host-csrf" {
		t.Errorf("production cookie is %q: without the prefix a subdomain can overwrite it and defeat double submit", got)
	}

	if got := newTestStore(false).csrfName(); got != "csrf" {
		t.Errorf("development cookie is %q: the __Host- prefix needs https and would be dropped over plain http", got)
	}
}

func TestAPageThatShowsAFormIsMarkedAsHavingUsedTheToken(t *testing.T) {
	store := newTestStore(false)
	w := httptest.NewRecorder()

	sess := &Session{Data: map[string]string{}, store: store, w: w}

	if sess.CSRFUsed() {
		t.Fatal("a page that rendered no form claims it used a token")
	}

	sess.CSRF()

	if !sess.CSRFUsed() {
		t.Error("a rendered form is not flagged: such a page could be handed to a shared cache with its Set-Cookie, giving every visitor one token")
	}
}
