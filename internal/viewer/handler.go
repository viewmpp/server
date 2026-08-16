package viewer

import (
	"fmt"
	"log/slog"
	"net/http"
	"server/internal/fixtures"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/user"
	"strings"
)

type Handler struct {
	templates *htmlutil.Templates
	baseURL   string
	logger    *slog.Logger
}

func NewHandler(
	templates *htmlutil.Templates,
	baseURL string,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		templates: templates,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		logger:    logger,
	}
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, fixtures.Examples())
	page.Description = "Open an MS Project .mpp or .xml file in your browser and read the Gantt chart straight away — tasks, dates, dependencies and the critical path, with no install and no signup."
	page.Canonical = h.baseURL + "/"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, page, h.logger)
}

func (h *Handler) ExamplesPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, fixtures.Examples())
	page.Description = "Open a sample MS Project plan in your browser — Gantt chart, task table and dependencies, with no file of your own and no signup."
	page.Canonical = h.baseURL + "/examples"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Examples, page, h.logger)
}

func (h *Handler) ExamplePage(w http.ResponseWriter, r *http.Request) {
	e, ok := fixtures.ByName(r.PathValue("name"))
	if !ok {
		htmlutil.NotFoundPage(w, r, h.logger)
		return
	}

	page := user.NewPage(r, nil)
	page.ExampleName = e.Name
	page.ExampleLabel = e.Label
	page.FileName = e.FileName
	page.Description = exampleDescription(e)
	page.Canonical = h.baseURL + "/example/" + e.Name
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, page, h.logger)
}

func (h *Handler) ExampleContract(w http.ResponseWriter, r *http.Request) {
	e, ok := fixtures.ByName(r.PathValue("name"))
	if !ok {
		jsonutil.NotFoundResponse(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(e.Contract)
}

func (h *Handler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, nil)
	page.Description = "What MPP Viewer stores, what it does not, and how long anything is kept. Uploaded files are never written to disk."
	page.Canonical = h.baseURL + "/privacy"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Privacy, page, h.logger)
}

func (h *Handler) TermsPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, nil)
	page.Description = "The terms for using MPP Viewer: what the service does, what you may upload, how sharing works and what the limits are."
	page.Canonical = h.baseURL + "/terms"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Terms, page, h.logger)
}

func exampleDescription(e fixtures.Example) string {
	return fmt.Sprintf(
		"%s — %s. Open this sample MS Project plan in the browser: Gantt chart, task table and dependencies, no install and no signup.",
		e.Label, e.Note)
}
