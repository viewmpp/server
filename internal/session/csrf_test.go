package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestStore(secure bool) *Store {
	return &Store{lifetime: 12 * time.Hour, secure: secure, secret: []byte("test-secret")}
}

func render(t *testing.T, store *Store, r *http.Request, bind string) (string, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w, csrf: store.readCSRF(r), bind: bind}

	return sess.CSRF(), w
}

func submit(t *testing.T, store *Store, cookie, field, bind string) bool {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(CSRFField+"="+field))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: store.csrfName(), Value: cookie})
	}

	sess := &Session{Data: map[string]string{}, store: store, csrf: store.readCSRF(r), bind: bind}

	return VerifyCSRF(sess, r)
}

func TestTheFormCarriesASignatureAndTheCookieCarriesTheSecretHalf(t *testing.T) {
	store := newTestStore(false)

	field, w := render(t, store, httptest.NewRequest(http.MethodGet, "/signin", nil), "")

	cookie := w.Result().Cookies()[0]

	if cookie.Value == field {
		t.Error("the form repeats the cookie verbatim: that is the naive double submit OWASP advises against")
	}

	if !cookie.HttpOnly {
		t.Error("the csrf cookie is readable from javascript")
	}

	if !submit(t, store, cookie.Value, field, "") {
		t.Error("the pair a form was rendered with does not verify")
	}
}

func TestRenderingAFormWritesNothingToTheSession(t *testing.T) {
	store := newTestStore(false)
	sess := &Session{Data: map[string]string{}, store: store, w: httptest.NewRecorder()}

	sess.CSRF()

	if len(sess.Data) != 0 {
		t.Fatalf("session data holds %v: a form page must not create server state", sess.Data)
	}
}

func TestTheCookieSurvivesAcrossRequests(t *testing.T) {
	store := newTestStore(false)

	_, w := render(t, store, httptest.NewRequest(http.MethodGet, "/signin", nil), "")

	next := httptest.NewRequest(http.MethodGet, "/signin", nil)
	for _, c := range w.Result().Cookies() {
		next.AddCookie(c)
	}

	if _, second := render(t, store, next, ""); len(second.Result().Cookies()) != 0 {
		t.Error("a second page hands out another cookie: the form left open in the first tab would stop working")
	}
}

func TestWhatIsAcceptedAndWhatIsNot(t *testing.T) {
	store := newTestStore(false)

	field, w := render(t, store, httptest.NewRequest(http.MethodGet, "/signin", nil), "")
	value := w.Result().Cookies()[0].Value

	other := newTestStore(false)
	other.secret = []byte("stolen-guess")

	_, bw := render(t, store, httptest.NewRequest(http.MethodGet, "/account", nil), "session-abc")
	boundValue := bw.Result().Cookies()[0].Value

	tests := []struct {
		name   string
		cookie string
		field  string
		bind   string
		want   bool
	}{
		{"the pair the form was rendered with", value, field, "", true},
		{"signature invented", value, "not-a-signature", "", false},
		{"cookie replaced by an attacker who can write cookies", "planted-value", field, "", false},
		{"signature made with another secret", value, mustSign(other, value, ""), "", false},
		{"token from a signed in session replayed anonymously", boundValue, field, "", false},
		{"token bound to another session", boundValue, mustSign(store, boundValue, "session-xyz"), "session-abc", false},
		{"token bound to its own session", boundValue, mustSign(store, boundValue, "session-abc"), "session-abc", true},
		{"no cookie", "", field, "", false},
		{"no form field", value, "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := submit(t, store, tc.cookie, tc.field, tc.bind); got != tc.want {
				t.Errorf("accepted = %v, want %v", got, tc.want)
			}
		})
	}
}

func mustSign(store *Store, value, bind string) string {
	return (&Session{store: store, bind: bind}).sign(value)
}

func TestAPageThatShowsAFormIsFlagged(t *testing.T) {
	store := newTestStore(false)
	sess := &Session{Data: map[string]string{}, store: store, w: httptest.NewRecorder()}

	if sess.CSRFUsed() {
		t.Fatal("a page that rendered no form claims it used a token")
	}

	sess.CSRF()

	if !sess.CSRFUsed() {
		t.Error("a rendered form is not flagged: a shared cache could hand one token to every visitor")
	}
}

func TestCookiesAreLockedToTheHostInProduction(t *testing.T) {
	prod, dev := newTestStore(true), newTestStore(false)

	if got := prod.csrfName(); got != "__Host-csrf" {
		t.Errorf("production csrf cookie is %q: without the prefix a subdomain can overwrite it", got)
	}

	if got := prod.name(); got != "__Host-session" {
		t.Errorf("production session cookie is %q: without the prefix a subdomain can overwrite it", got)
	}

	if got := dev.csrfName(); got != "csrf" {
		t.Errorf("development csrf cookie is %q: the prefix needs https", got)
	}

	if got := dev.name(); got != "session" {
		t.Errorf("development session cookie is %q: the prefix needs https", got)
	}
}
