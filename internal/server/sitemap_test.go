package server

import (
	"net/http"
	"net/http/httptest"
	"server/internal/examples"
	"strings"
	"testing"
)

func TestSitemapCoversPublicPages(t *testing.T) {
	paths := sitemapPaths()

	if len(paths) == 0 || paths[0] != "/" {
		t.Fatalf("the landing page must come first, got %v", paths)
	}

	for _, e := range examples.All() {
		want := "/example/" + e.Name
		if !contains(paths, want) {
			t.Errorf("example page %s is missing from the sitemap", want)
		}
	}
}

func TestSitemapExcludesPrivatePages(t *testing.T) {
	forbidden := []string{"/p/", "/projects", "/signin", "/signup", "/verify", "/reset", "/api/"}

	for _, path := range sitemapPaths() {
		for _, bad := range forbidden {
			if strings.HasPrefix(path, bad) {
				t.Errorf("%s must not be listed in the sitemap", path)
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func sitemapBody(t *testing.T) string {
	t.Helper()

	s := newTestServer(t)
	s.cfg.AppBaseURL = "https://viewmpp.com"

	w := httptest.NewRecorder()
	s.sitemap(w, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	return w.Body.String()
}

func TestSitemapClaimsNoModificationDate(t *testing.T) {
	body := sitemapBody(t)

	if strings.Contains(body, "lastmod") {
		t.Error("a date stamped on every url at once is not a modification date; an absent field beats an untrue one")
	}

	for _, path := range sitemapPaths() {
		want := "<loc>https://viewmpp.com" + path + "</loc>"
		if !strings.Contains(body, want) {
			t.Errorf("%s is missing from the sitemap", path)
		}
	}
}
