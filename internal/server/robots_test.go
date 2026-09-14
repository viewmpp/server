package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRobotsBlocksTransactionalPages(t *testing.T) {
	transactional := []string{
		"/signin",
		"/signup",
		"/verify",
		"/reset",
		"/reset/8f3a",
		"/p/abc123",
	}

	for _, path := range transactional {
		if crawlable(path) {
			t.Errorf("%s exposes a transactional form and should not be crawled", path)
		}
	}
}

func TestRobotsLeavesThePublicPagesCrawlable(t *testing.T) {
	for _, path := range sitemapPaths() {
		if !crawlable(path) {
			t.Errorf("%s is offered in the sitemap but blocked by robots.txt", path)
		}
	}
}

func robotsBody(t *testing.T) string {
	t.Helper()

	s := newTestServer(t)
	s.cfg.AppBaseURL = "https://viewmpp.com"

	w := httptest.NewRecorder()
	s.robots(w, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	return w.Body.String()
}

type robotsGroup struct {
	agents    []string
	disallows []string
}

func parseRobots(t *testing.T, body string) []robotsGroup {
	t.Helper()

	var groups []robotsGroup
	var current *robotsGroup

	for _, line := range strings.Split(body, "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			current = nil
			continue
		}

		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		switch name {
		case "User-agent":
			if current == nil {
				groups = append(groups, robotsGroup{})
				current = &groups[len(groups)-1]
			}
			current.agents = append(current.agents, value)
		case "Disallow":
			if current == nil {
				t.Fatalf("Disallow outside a group: %q", line)
			}
			current.disallows = append(current.disallows, value)
		}
	}

	return groups
}

func TestEveryAgentGroupKeepsThePrivatePathsOut(t *testing.T) {
	groups := parseRobots(t, robotsBody(t))

	if len(groups) != len(agentGroups()) {
		t.Fatalf("parsed %d groups, wrote %d", len(groups), len(agentGroups()))
	}

	for _, group := range groups {
		t.Run(strings.Join(group.agents, ","), func(t *testing.T) {
			for _, path := range disallowed {
				if !contains(group.disallows, path) {
					t.Errorf("%s is not disallowed for %v: a named group inherits nothing from the wildcard one",
						path, group.agents)
				}
			}
		})
	}
}

func TestNoAgentIsNamedTwice(t *testing.T) {
	seen := map[string]bool{}

	for _, group := range agentGroups() {
		for _, agent := range group {
			if seen[agent] {
				t.Errorf("%s appears in two groups, so which rules apply to it is undefined", agent)
			}
			seen[agent] = true
		}
	}
}

func TestTheAssistantsAreAddressedByName(t *testing.T) {
	body := robotsBody(t)

	for _, agent := range append(append([]string{}, trainingAgents...), assistantAgents...) {
		if !strings.Contains(body, "User-agent: "+agent+"\n") {
			t.Errorf("%s is not addressed by name", agent)
		}
	}

	if !strings.HasSuffix(body, "Sitemap: https://viewmpp.com/sitemap.xml\n") {
		t.Error("the sitemap line must survive at the end of the file")
	}
}
