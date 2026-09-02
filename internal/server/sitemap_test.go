package server

import (
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
