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

func render(t *testing.T, store *Store, r *http.Request, token string) (string, *httptest.ResponseRecorder) {
	t.Helper()

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w, csrf: store.readCSRF(r), Token: token}

	if c, err := r.Cookie(store.name()); err == nil {
		sess.Token, sess.sent, sess.stored = c.Value, true, true
	}

	return sess.CSRF(), w
}

func submit(t *testing.T, store *Store, cookie, field, token string, userID *int64) bool {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/signin", strings.NewReader(CSRFField+"="+field))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: store.csrfName(), Value: cookie})
	}

	sess := &Session{Data: map[string]string{}, store: store, csrf: store.readCSRF(r), Token: token, UserID: userID}

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

	if !submit(t, store, cookie.Value, field, "", nil) {
		t.Error("the pair a form was rendered with does not verify")
	}
}

func TestRenderingAnAnonymousFormDoesNotEstablishASession(t *testing.T) {
	store := newTestStore(false)
	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w}

	sess.CSRF()

	if len(sess.Data) != 0 {
		t.Errorf("session data holds %v: a form page must not write application state", sess.Data)
	}

	if sess.Unsaved() {
		t.Error("the form marked the anonymous session for persistence")
	}

	var issued []string
	for _, c := range w.Result().Cookies() {
		issued = append(issued, c.Name)
	}

	if len(issued) != 1 || issued[0] != store.csrfName() {
		t.Errorf("cookies issued: %v, want only %s", issued, store.csrfName())
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

	userID := int64(42)
	boundWriter := httptest.NewRecorder()
	boundSession := &Session{Data: map[string]string{}, store: store, w: boundWriter, Token: "session-abc", UserID: &userID, sent: true, stored: true}
	boundField := boundSession.CSRF()
	boundValue := boundWriter.Result().Cookies()[0].Value

	tests := []struct {
		name   string
		cookie string
		field  string
		token  string
		userID *int64
		want   bool
	}{
		{"the anonymous pair the form was rendered with", value, field, "", nil, true},
		{"signature invented", value, "not-a-signature", "", nil, false},
		{"cookie replaced by an attacker who can write cookies", "planted-value", field, "", nil, false},
		{"signature made with another secret", value, mustSign(other, value, "", nil), "", nil, false},
		{"an anonymous signature used after sign in", value, field, "session-abc", &userID, false},
		{"a signature made for a different cookie", boundValue, field, "", nil, false},
		{"an authenticated token replayed anonymously", boundValue, boundField, "", nil, false},
		{"a token minted for another visitor's session", boundValue, mustSign(store, boundValue, "session-xyz", &userID), "session-abc", &userID, false},
		{"a token minted for this authenticated session", boundValue, boundField, "session-abc", &userID, true},
		{"no cookie", "", field, "", nil, false},
		{"no form field", value, "", "", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := submit(t, store, tc.cookie, tc.field, tc.token, tc.userID); got != tc.want {
				t.Errorf("accepted = %v, want %v", got, tc.want)
			}
		})
	}
}

func mustSign(store *Store, value, token string, userID *int64) string {
	return (&Session{store: store, Token: token, UserID: userID}).sign(value)
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

func TestSigningInInvalidatesAFormRenderedBeforeIt(t *testing.T) {
	store := newTestStore(false)

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w}

	stale := sess.CSRF()
	staleCookie := w.Result().Cookies()[0].Value

	const token = "a-freshly-issued-session-token"
	userID := int64(42)

	renewed := httptest.NewRecorder()
	sess.rotate(token)
	sess.UserID = &userID
	store.rotateCSRF(renewed, sess)

	freshCookie := renewed.Result().Cookies()[0].Value

	if submit(t, store, freshCookie, stale, token, &userID) {
		t.Error("a form rendered before sign in still verifies afterwards: the security context changed and the old token must die with it")
	}

	if submit(t, store, staleCookie, stale, token, &userID) {
		t.Error("the old pair verifies against the new session: binding is not in force")
	}
}

func TestRenewalHandsOutAFreshPair(t *testing.T) {
	store := newTestStore(false)

	w := httptest.NewRecorder()
	sess := &Session{Data: map[string]string{}, store: store, w: w, ExpiresAt: time.Now().Add(time.Hour)}

	before := sess.CSRF()
	sess.rotate("new-session-token")
	userID := int64(42)
	sess.UserID = &userID

	renewed := httptest.NewRecorder()
	store.rotateCSRF(renewed, sess)

	cookies := renewed.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("renewal handed out no csrf cookie: the next form on the page would have nothing to match")
	}

	after := sess.CSRF()

	if before == after {
		t.Error("the token survived renewal unchanged")
	}

	if !submit(t, store, cookies[0].Value, after, sess.Token, &userID) {
		t.Error("the pair issued during renewal does not verify")
	}
}
