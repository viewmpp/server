package server

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder

	b.WriteString("User-agent: *\n")
	b.WriteString("Disallow: /p/\n")
	b.WriteString("\n")
	_, err := fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", strings.TrimSuffix(s.cfg.BaseURL, "/"))
	if err != nil {
		s.logger.Error("formating error", "err", err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(b.String()))
}
