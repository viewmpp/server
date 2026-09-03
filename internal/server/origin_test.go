package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func requestFrom(t *testing.T, s *Server, method, path, origin, referer string) int {
	t.Helper()

	r := httptest.NewRequest(method, path, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if referer != "" {
		r.Header.Set("Referer", referer)
	}

	w := httptest.NewRecorder()

	s.sameOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(w, r)

	return w.Code
}

func originServer(t *testing.T) *Server {
	t.Helper()

	s := newTestServer(t)
	s.cfg.AppBaseURL = "https://viewmpp.com"

	return s
}

func TestPostsFromAnotherSiteAreRefused(t *testing.T) {
	s := originServer(t)

	tests := []struct {
		name    string
		path    string
		origin  string
		referer string
		want    int
	}{
		{"our own form", "/p/abc/access", "https://viewmpp.com", "", http.StatusOK},
		{"another site", "/p/abc/access", "https://evil.example", "", http.StatusBadRequest},
		{"another site calling the api", "/api/v1/projects", "https://evil.example", "", http.StatusForbidden},
		{"a lookalike host", "/p/abc/access", "https://viewmpp.com.evil.example", "", http.StatusBadRequest},
		{"plain http version of us", "/p/abc/access", "http://viewmpp.com", "", http.StatusBadRequest},
		{"referer used when origin is absent", "/p/abc/access", "", "https://evil.example/page", http.StatusBadRequest},
		{"our own referer", "/p/abc/access", "", "https://viewmpp.com/p/abc", http.StatusOK},
		{"neither header, as curl and old proxies send", "/p/abc/access", "", "", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := requestFrom(t, s, http.MethodPost, tc.path, tc.origin, tc.referer); got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReadingIsNeverBlockedByOrigin(t *testing.T) {
	s := originServer(t)

	if got := requestFrom(t, s, http.MethodGet, "/p/abc", "https://evil.example", ""); got != http.StatusOK {
		t.Errorf("a plain read was refused with %d: links arriving from other sites are how sharing works", got)
	}
}
