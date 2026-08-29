package server

import "testing"

func TestRobotsBlocksThePagesThatMintSessions(t *testing.T) {
	minting := []string{
		"/signin",
		"/signup",
		"/verify",
		"/reset",
		"/reset/8f3a",
		"/p/abc123",
	}

	for _, path := range minting {
		if crawlable(path) {
			t.Errorf("%s renders a form to anonymous visitors, so every cookieless crawl of it writes a session row", path)
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
