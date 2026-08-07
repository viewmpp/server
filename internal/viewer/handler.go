package viewer

import (
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/htmlutil"
	"server/internal/session"
	"server/internal/user"
	"server/internal/vcs"
)

type Handler struct {
	templates *htmlutil.Templates
	logger    *slog.Logger
}

func NewHandler(
	templates *htmlutil.Templates,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		templates: templates,
		logger:    logger,
	}
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)
	u := user.GetUserContext(r)

	page := htmlutil.Page{
		MaxUpload: u.MaxUploadBytes(),
		Version:   url.QueryEscape(vcs.Version()),
		Flash:     sess.Pop("flash"),
		CSRFToken: sess.CSRF(),
	}

	if !u.IsAnonymous() {
		page.UserEmail = u.Email
		page.Verified = u.Verified
	}

	err := htmlutil.WriteHTML(w, http.StatusOK, h.templates.App, page)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
