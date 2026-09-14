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

var everyAgent = []string{"*"}

var trainingAgents = []string{
	"GPTBot",
	"ClaudeBot",
	"Google-Extended",
	"Applebot-Extended",
	"CCBot",
}

var assistantAgents = []string{
	"OAI-SearchBot",
	"ChatGPT-User",
	"Claude-SearchBot",
	"Claude-User",
	"PerplexityBot",
	"Perplexity-User",
}

func agentGroups() [][]string {
	return [][]string{everyAgent, trainingAgents, assistantAgents}
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder

	for _, agents := range agentGroups() {
		writeGroup(&b, agents)
	}

	_, _ = fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", strings.TrimSuffix(s.cfg.AppBaseURL, "/"))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(b.String()))
}

func writeGroup(b *strings.Builder, agents []string) {
	for _, agent := range agents {
		_, _ = fmt.Fprintf(b, "User-agent: %s\n", agent)
	}

	b.WriteString("Allow: /\n")

	for _, path := range disallowed {
		_, _ = fmt.Fprintf(b, "Disallow: %s\n", path)
	}

	b.WriteString("\n")
}

func crawlable(path string) bool {
	for _, prefix := range disallowed {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}

	return true
}
