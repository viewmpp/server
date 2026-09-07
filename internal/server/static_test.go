package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serveStatic(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()

	s := &Server{}
	s.cfg.AppEnv = "prod"

	w := httptest.NewRecorder()
	s.static().ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

	return w
}

func TestStaticRefusesEverythingOutsideItsOwnDirectory(t *testing.T) {
	targets := []string{
		"/static/%2e%2e/templates/base.tmpl",
		"/static/..%2ftemplates/base.tmpl",
		"/static/%2e%2e/templates/emails/email_verification.tmpl",
		"/static/js/%2e%2e/%2e%2e/templates/base.tmpl",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			w := serveStatic(t, target)

			if w.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404: a template was reachable through the asset route", w.Code)
			}

			if w.Body.Len() > 0 && w.Body.String() != "404 page not found\n" {
				t.Errorf("body leaked %d bytes of something that is not the 404 page", w.Body.Len())
			}
		})
	}
}

func TestStaticRefusesDirectories(t *testing.T) {
	for _, target := range []string{"/static/", "/static/js/", "/static/icons/"} {
		t.Run(target, func(t *testing.T) {
			if code := serveStatic(t, target).Code; code != http.StatusNotFound {
				t.Errorf("status = %d, want 404: a directory listing was served", code)
			}
		})
	}
}

func TestStaticStillServesAssets(t *testing.T) {
	w := serveStatic(t, "/static/css/app.css")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if w.Body.Len() == 0 {
		t.Error("asset came back empty")
	}

	if got := w.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want the long-lived value outside development", got)
	}
}
