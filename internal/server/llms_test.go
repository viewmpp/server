package server

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
	"testing"
)

var llmsLink = regexp.MustCompile(`\]\(([^)]+)\)`)

func llmsBody(t *testing.T) string {
	t.Helper()

	s := newTestServer(t)
	s.cfg.AppBaseURL = "https://viewmpp.com"

	w := httptest.NewRecorder()
	s.llms(w, httptest.NewRequest(http.MethodGet, "/llms.txt", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q: a browser must be able to read this file, not download it", got)
	}

	return w.Body.String()
}

func llmsPaths(t *testing.T, body string) []string {
	t.Helper()

	const base = "https://viewmpp.com"

	var paths []string

	for _, found := range llmsLink.FindAllStringSubmatch(body, -1) {
		url := found[1]

		if !strings.HasPrefix(url, base) {
			t.Errorf("%s is not an absolute address on this site", url)
			continue
		}

		paths = append(paths, strings.TrimPrefix(url, base))
	}

	if len(paths) == 0 {
		t.Fatal("no links at all")
	}

	return paths
}

func TestLLMsOffersNothingRobotsForbids(t *testing.T) {
	for _, path := range llmsPaths(t, llmsBody(t)) {
		if !crawlable(path) {
			t.Errorf("%s is offered to assistants and blocked in robots.txt: the two files must not disagree", path)
		}
	}
}

func TestLLMsListsThePublicPagesOnce(t *testing.T) {
	paths := llmsPaths(t, llmsBody(t))

	seen := map[string]int{}
	for _, path := range paths {
		seen[path]++
	}

	for _, page := range landing.All() {
		switch seen[page.Slug] {
		case 1:
		case 0:
			t.Errorf("%s is missing: a page worth indexing is worth naming here", page.Slug)
		default:
			t.Errorf("%s is listed %d times", page.Slug, seen[page.Slug])
		}
	}

	for _, e := range examples.All() {
		if seen["/example/"+e.Name] != 1 {
			t.Errorf("example %s is not listed exactly once", e.Name)
		}
	}
}

func TestLLMsStartsWithATitleAndASummary(t *testing.T) {
	lines := strings.Split(llmsBody(t), "\n")

	if lines[0] != "# View MPP" {
		t.Errorf("first line = %q, want a single H1 naming the site", lines[0])
	}

	if !strings.HasPrefix(lines[2], "> ") {
		t.Errorf("third line = %q, want the one-line summary as a blockquote", lines[2])
	}
}
