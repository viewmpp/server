package server

import "testing"

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
