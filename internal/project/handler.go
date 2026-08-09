package project

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/session"
	"server/internal/user"
	"server/internal/vcs"
)

type Handler struct {
	store     *Store
	templates *htmlutil.Templates
	logger    *slog.Logger
}

func NewHandler(store *Store, templates *htmlutil.Templates, logger *slog.Logger) *Handler {
	return &Handler{
		store:     store,
		templates: templates,
		logger:    logger,
	}
}

func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, false)
	if !ok {
		return
	}

	sess := session.FromContext(r)
	u := user.GetUserContext(r)

	page := htmlutil.Page{
		MaxUpload: u.MaxUploadBytes(),
		Version:   url.QueryEscape(vcs.Version()),
		Flash:     sess.Pop("flash"),
		CSRFToken: sess.CSRF(),
		ProjectID: p.PublicID,
		FileName:  p.FileName,
	}

	if !u.IsAnonymous() {
		page.UserEmail = u.Email
		page.Verified = u.Verified
	}

	if err := htmlutil.WriteHTML(w, http.StatusOK, h.templates.App, page); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}

func (h *Handler) Contract(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, true)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(p.Contract)
}

func (h *Handler) find(w http.ResponseWriter, r *http.Request, asJSON bool) (*Project, bool) {
	p, err := h.store.GetByPublicID(r.Context(), r.PathValue("id"))
	if err != nil && !errors.Is(err, ErrNotFound) {
		if asJSON {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
		} else {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
		}
		return nil, false
	}

	u := user.GetUserContext(r)

	if p == nil || (!p.IsPublic() && (u.IsAnonymous() || u.ID != p.UserID)) {
		if asJSON {
			jsonutil.NotFoundResponse(w)
		} else {
			htmlutil.NotFoundPage(w, r, h.logger)
		}
		return nil, false
	}

	return p, true
}

const listLimit = 100

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	projects, err := h.store.ListByUserID(r.Context(), u.ID, listLimit)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess := session.FromContext(r)

	page := htmlutil.Page{
		Version:   url.QueryEscape(vcs.Version()),
		Flash:     sess.Pop("flash"),
		CSRFToken: sess.CSRF(),
		UserEmail: u.Email,
		Verified:  u.Verified,
		Form:      projects,
	}

	if err = htmlutil.WriteHTML(w, http.StatusOK, h.templates.Projects, page); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
