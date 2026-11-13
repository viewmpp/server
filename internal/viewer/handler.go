package viewer

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"server/internal/events"
	"server/internal/examples"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/landing"
	"server/internal/user"
	"strings"
	"time"
)

type counter interface {
	Count(name events.Name)
}

type trials interface {
	TrialOpen(ctx context.Context, userID int64) (bool, error)
}

type Handler struct {
	templates *htmlutil.Templates
	baseURL   string
	trials    trials
	counts    counter
	logger    *slog.Logger
}

func NewHandler(
	templates *htmlutil.Templates,
	baseURL string,
	trials trials,
	counts counter,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		templates: templates,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		trials:    trials,
		counts:    counts,
		logger:    logger,
	}
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, examples.All())
	page.Description = landing.BySlug("/").Description
	page.Canonical = h.baseURL + "/"
	page.Slug = "/"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, page, h.logger)
}

func (h *Handler) ExamplesPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, examples.All())
	page.Description = landing.BySlug("/examples").Description
	page.Canonical = h.baseURL + "/examples"
	page.Slug = "/examples"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Examples, page, h.logger)
}

func (h *Handler) ExamplePage(w http.ResponseWriter, r *http.Request) {
	e, ok := examples.ByName(r.PathValue("name"))
	if !ok {
		htmlutil.NotFoundPage(w, r, h.logger)
		return
	}

	page := user.NewPage(r, e)
	page.ExampleName = e.Name
	page.ExampleLabel = e.Label
	page.FileName = e.FileName
	page.Description = e.Description()
	page.Canonical = h.baseURL + "/example/" + e.Name
	page.Slug = "/example/" + e.Name
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Example, page, h.logger)
}

func (h *Handler) ExampleContract(w http.ResponseWriter, r *http.Request) {
	e, ok := examples.ByName(r.PathValue("name"))
	if !ok {
		jsonutil.NotFoundResponse(w)
		return
	}

	now := time.Now()

	plan, err := e.Contract(now)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.counts.Count(events.ExampleOpen)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(time.Until(examples.NextWeek(now)).Seconds())))
	_, _ = w.Write(plan)
}

func (h *Handler) CookiesPage(w http.ResponseWriter, r *http.Request) {
	h.landing(w, r, "/cookies", h.templates.Cookies)
}

func (h *Handler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, nil)
	page.Description = landing.BySlug("/privacy").Description
	page.Canonical = h.baseURL + "/privacy"
	page.Slug = "/privacy"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Privacy, page, h.logger)
}

func (h *Handler) TermsPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, nil)
	page.Description = landing.BySlug("/terms").Description
	page.Canonical = h.baseURL + "/terms"
	page.Slug = "/terms"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Terms, page, h.logger)
}

func (h *Handler) WithoutProjectPage(w http.ResponseWriter, r *http.Request) {
	h.landing(w, r, "/open-mpp-file-without-ms-project", h.templates.WithoutProject)
}

func (h *Handler) MacPage(w http.ResponseWriter, r *http.Request) {
	h.landing(w, r, "/open-mpp-file-on-mac", h.templates.Mac)
}

func (h *Handler) XERPage(w http.ResponseWriter, r *http.Request) {
	h.landing(w, r, "/open-xer-file-without-primavera", h.templates.XER)
}

func (h *Handler) XMLPage(w http.ResponseWriter, r *http.Request) {
	h.landing(w, r, "/open-microsoft-project-xml", h.templates.XML)
}

func (h *Handler) SharePage(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Header.Get("Referer"), h.baseURL+"/p/") {
		h.counts.Count(events.SharePageFromLink)
	}

	h.landing(w, r, "/share-a-project-plan", h.templates.Share)
}

func (h *Handler) PricingPage(w http.ResponseWriter, r *http.Request) {
	page := user.NewPage(r, nil)

	u := user.GetUserContext(r)
	if page.Billing && !u.IsAnonymous() && !page.Pro && h.trials != nil {
		open, err := h.trials.TrialOpen(r.Context(), u.ID)
		if err != nil {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
		page.TrialOpen = open
	}

	h.landingPage(w, r, "/pricing", h.templates.Pricing, page)
}

func (h *Handler) landing(w http.ResponseWriter, r *http.Request, slug string, tmpl *template.Template) {
	h.landingPage(w, r, slug, tmpl, user.NewPage(r, nil))
}

func (h *Handler) landingPage(w http.ResponseWriter, r *http.Request, slug string, tmpl *template.Template, page htmlutil.Page) {
	c := landing.BySlug(slug)

	page.Description = c.Description
	page.Canonical = h.baseURL + c.Slug
	page.Slug = c.Slug
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, tmpl, page, h.logger)
}
