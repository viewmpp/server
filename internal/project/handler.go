package project

import (
	"errors"
	"log/slog"
	"net/http"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/user"
)

type Handler struct {
	store     *Store
	templates *htmlutil.Templates
	logger    *slog.Logger
}

func NewHandler(
	store *Store,
	templates *htmlutil.Templates,
	logger *slog.Logger,
) *Handler {
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

	u := user.GetUserContext(r)

	page := user.NewPage(r, nil)
	page.ProjectID = p.PublicID
	page.FileName = p.FileName
	page.Access = p.Access
	page.IsOwner = !u.IsAnonymous() && u.ID == p.UserID
	page.CanShare = u.CanShare()

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, page, h.logger)
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

func (h *Handler) SetAccess(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	publicID := r.PathValue("id")

	access := r.PostFormValue("access")
	if access != AccessPublic && access != AccessPrivate {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if access == AccessPublic && !u.CanShare() {
		sess.Put("flash", MsgShareNeedsPro)
		http.Redirect(w, r, "/p/"+publicID, http.StatusSeeOther)
		return
	}

	if err := h.store.SetAccess(r.Context(), publicID, u.ID, access); err != nil {
		if errors.Is(err, ErrNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if access == AccessPublic {
		sess.Put("flash", MsgNowPublic)
	} else {
		sess.Put("flash", MsgNowPrivate)
	}

	http.Redirect(w, r, "/p/"+publicID, http.StatusSeeOther)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	if err := h.store.Delete(r.Context(), r.PathValue("id"), u.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.Put("flash", MsgProjectDeleted)

	http.Redirect(w, r, "/projects", http.StatusSeeOther)
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
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	projects, err := h.store.ListByUserID(r.Context(), u.ID, listLimit)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Projects, user.NewPage(r, projects), h.logger)
}
