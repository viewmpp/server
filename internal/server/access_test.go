package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"server/internal/user"
)

var someone = &user.User{ID: 7, Email: "someone@example.com"}

func dispatch(t *testing.T, method, path string, as *user.User, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	s := newTestServer(t)

	r := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		r.Header.Set(name, value)
	}

	w := httptest.NewRecorder()
	s.mux().ServeHTTP(w, user.SetUserContext(r, as))

	return w
}

func TestSignedOutVisitorsAreSentToSignIn(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/projects"},
		{http.MethodGet, "/account"},
		{http.MethodPost, "/account/password"},
		{http.MethodPost, "/account/delete"},
		{http.MethodGet, "/verify"},
		{http.MethodPost, "/verify"},
		{http.MethodPost, "/verify/resend"},
		{http.MethodPost, "/subscribe"},
		{http.MethodPost, "/p/abc/access"},
		{http.MethodPost, "/p/abc/delete"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			w := dispatch(t, route.method, route.path, user.AnonymousUser, nil)

			if w.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d: the route is not behind requireAuthUser", w.Code, http.StatusSeeOther)
			}

			if got := w.Header().Get("Location"); got != "/signin" {
				t.Errorf("Location = %q, want /signin", got)
			}
		})
	}
}

func TestSignedInVisitorsSkipTheEntrancePages(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/signin"},
		{http.MethodPost, "/signin"},
		{http.MethodGet, "/signup"},
		{http.MethodPost, "/signup"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			w := dispatch(t, route.method, route.path, someone, nil)

			if w.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d: a signed-in visitor should not see this page", w.Code, http.StatusSeeOther)
			}

			if got := w.Header().Get("Location"); got != "/" {
				t.Errorf("Location = %q, want /", got)
			}
		})
	}
}

func TestTheAPIAnswersWithJSONRatherThanARedirect(t *testing.T) {
	app := map[string]string{appHeader: appHeaderValue}

	w := dispatch(t, http.MethodPost, "/api/v1/projects", user.AnonymousUser, app)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d: an XHR that follows a redirect gets HTML where it expects JSON", w.Code, http.StatusUnauthorized)
	}

	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	if w.Header().Get("Location") != "" {
		t.Error("the API answered with a redirect")
	}
}

func TestTheAppHeaderIsCheckedBeforeTheUser(t *testing.T) {
	w := dispatch(t, http.MethodPost, "/api/v1/projects", user.AnonymousUser, nil)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d: a call without the app header should never reach the auth guard", w.Code, http.StatusForbidden)
	}
}

func TestTheGuardsThemselvesLetThroughWhoTheyShould(t *testing.T) {
	s := newTestServer(t)

	reached := false
	next := func(w http.ResponseWriter, r *http.Request) { reached = true }

	for name, c := range map[string]struct {
		handler http.HandlerFunc
		as      *user.User
	}{
		"requireAuthUser with a user":         {s.requireAuthUser(next), someone},
		"requireAuthAPI with a user":          {s.requireAuthAPI(next), someone},
		"requireAnonymousUser with a visitor": {s.requireAnonymousUser(next), user.AnonymousUser},
	} {
		t.Run(name, func(t *testing.T) {
			reached = false

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			c.handler(httptest.NewRecorder(), user.SetUserContext(r, c.as))

			if !reached {
				t.Error("the guard blocked a request it was supposed to pass through")
			}
		})
	}
}
