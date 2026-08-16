package server

import (
	"encoding/xml"
	"net/http"
	"server/internal/fixtures"
	"server/internal/landing"
	"server/internal/vcs"
	"strings"
	"time"
)

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func sitemapPaths() []string {
	paths := make([]string, 0, len(landing.All()))
	for _, page := range landing.All() {
		paths = append(paths, page.Slug)
	}

	for _, e := range fixtures.Examples() {
		paths = append(paths, "/example/"+e.Name)
	}

	return append(paths, "/privacy", "/terms")
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSuffix(s.cfg.BaseURL, "/")

	set := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}

	var lastMod string
	if at, ok := vcs.Time(); ok {
		lastMod = at.UTC().Format(time.DateOnly)
	}

	for _, path := range sitemapPaths() {
		set.URLs = append(set.URLs, sitemapURL{Loc: base + path, LastMod: lastMod})
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		s.logger.Error("sitemap not built", "err", err)
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n"))
}
