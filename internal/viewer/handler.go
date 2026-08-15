package viewer

import (
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
	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, user.NewPage(r, fixtures.Examples()), h.logger)
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
