package server

import (
	"fmt"
	"net/http"
	"strings"
)

var disallowed = []string{
	"/p/",
	"/signin",
	"/signup",
	"/verify",
	"/reset",
	"/account",
	"/projects",
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder

	b.WriteString("User-agent: *\n")

	for _, path := range disallowed {
		_, _ = fmt.Fprintf(&b, "Disallow: %s\n", path)
	}

	b.WriteString("\n")
	_, _ = fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", strings.TrimSuffix(s.cfg.BaseURL, "/"))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(b.String()))
}

func crawlable(path string) bool {
	for _, prefix := range disallowed {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}

	return true
}
