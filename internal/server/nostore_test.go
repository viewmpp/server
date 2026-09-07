package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func cachingHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(body))
	})
}

func serveNoStore(t *testing.T, env string, next http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	s := &Server{}
	s.cfg.AppEnv = env

	w := httptest.NewRecorder()
	s.noStore(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))

	return w
}

func TestDevelopmentOverridesCachingEvenWithoutWriteHeader(t *testing.T) {
	w := serveNoStore(t, "dev", cachingHandler("body"))

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a handler that writes without calling WriteHeader slipped past the development override", got)
	}

	if w.Body.String() != "body" {
		t.Errorf("body = %q, want %q: the wrapper must pass writes through untouched", w.Body.String(), "body")
	}
}

func TestDevelopmentOverridesCachingWithWriteHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusTeapot)
	})

	w := serveNoStore(t, "dev", handler)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d: the wrapper must not change the status a handler chose", w.Code, http.StatusTeapot)
	}
}

func TestOutsideDevelopmentTheHandlerKeepsItsOwnCaching(t *testing.T) {
	w := serveNoStore(t, "prod", cachingHandler("body"))

	if got := w.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want the handler's own value: no-store must never reach production", got)
	}
}
