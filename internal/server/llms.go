package server

import (
	"fmt"
	"net/http"
	"server/internal/examples"
	"server/internal/landing"
	"strings"
)

const llmsSummary = "Open an .mpp, Microsoft Project .xml, .mpx, .mpd or Primavera .xer schedule in the browser and read the Gantt chart at once - tasks, dates, dependencies, critical path. Nothing to install, no signup."

const llmsBoundary = `View MPP is a reader, not an editor. Nothing here changes a schedule and no date is recalculated: the critical path, the summary dates and the calendars are read out of the file exactly as Microsoft Project or Primavera P6 wrote them. An uploaded file is parsed in memory and never written to disk, and without an account nothing about it survives the request. Reading a plan, exporting it to a spreadsheet and opening a link somebody shared are free and need no account; an account exists only to keep plans and to share them.`

var llmsSections = []struct {
	title string
	group landing.Group
}{
	{"Open a schedule", landing.GroupView},
	{"Convert", landing.GroupConvert},
	{"Product", landing.GroupProduct},
	{"Policies", landing.GroupLegal},
}

func (s *Server) llms(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSuffix(s.cfg.AppBaseURL, "/")

	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "# View MPP\n\n> %s\n\n%s\n", llmsSummary, llmsBoundary)

	if home := landing.BySlug("/"); crawlable(home.Slug) {
		_, _ = fmt.Fprintf(&b, "\n## Start here\n\n- [Open a file](%s/): %s\n", base, home.Description)
	}

	for _, section := range llmsSections {
		lines := groupLines(base, section.group)
		if len(lines) == 0 {
			continue
		}

		_, _ = fmt.Fprintf(&b, "\n## %s\n\n%s", section.title, strings.Join(lines, ""))
	}

	if lines := exampleLines(base); len(lines) > 0 {
		_, _ = fmt.Fprintf(&b, "\n## Example plans\n\n%s", strings.Join(lines, ""))
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(b.String()))
}

func groupLines(base string, group landing.Group) []string {
	var lines []string

	for _, page := range landing.All() {
		if page.Group != group || !crawlable(page.Slug) {
			continue
		}

		lines = append(lines, fmt.Sprintf("- [%s](%s%s): %s\n", page.Label, base, page.Slug, page.Description))
	}

	return lines
}

func exampleLines(base string) []string {
	var lines []string

	for _, e := range examples.All() {
		path := "/example/" + e.Name
		if !crawlable(path) {
			continue
		}

		lines = append(lines, fmt.Sprintf("- [%s](%s%s): %s.\n", e.Label, base, path, e.Note))
	}

	return lines
}
