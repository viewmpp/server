package htmlutil

import (
	"html/template"
	"regexp"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
	"testing"
)

const minClusterLinks = 3

var (
	mainBlock = regexp.MustCompile(`(?s)<main[^>]*>(.*?)</main>`)
	crumbNav  = regexp.MustCompile(`(?s)<nav class="crumbs".*?</nav>`)
	hrefs     = regexp.MustCompile(`href="(/[^"]*)"`)
)

var offCluster = []string{"/static", "/signin", "/signup", "/projects", "/account", "/p/", "/verify", "/reset"}

func bodyLinks(t *testing.T, tmpl *template.Template, slug string) []string {
	t.Helper()

	page := Page{Slug: slug, Description: "x", Public: true, Examples: examples.All()}
	if slug == "/" || slug == "/examples" {
		page.Form = examples.All()
	}

	out := renderPage(t, tmpl, page)

	body := mainBlock.FindStringSubmatch(out)
	if body == nil {
		t.Fatalf("%s has no main element", slug)
	}

	seen := map[string]bool{}

	for _, found := range hrefs.FindAllStringSubmatch(crumbNav.ReplaceAllString(body[1], ""), -1) {
		target := found[1]

		if target == slug || beyondTheCluster(target) {
			continue
		}

		seen[target] = true
	}

	targets := make([]string, 0, len(seen))
	for target := range seen {
		targets = append(targets, target)
	}

	return targets
}

func beyondTheCluster(target string) bool {
	for _, prefix := range offCluster {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func TestClusterPagesLinkToTheirNeighbours(t *testing.T) {
	withBaseURL(t)

	for slug, tmpl := range indexable(t) {
		if landing.BySlug(slug).Group == landing.GroupLegal {
			continue
		}

		t.Run(slug, func(t *testing.T) {
			links := bodyLinks(t, tmpl, slug)

			if len(links) < minClusterLinks {
				t.Errorf("%s links to %d other pages, want at least %d: without them the cluster is a set of unconnected pages\n  %v",
					slug, len(links), minClusterLinks, links)
			}
		})
	}
}
