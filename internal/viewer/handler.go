package viewer

import (
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/htmlutil"
	"server/internal/user"
	"server/internal/vcs"
)

type Handler struct {
	logger    *slog.Logger
	templates *htmlutil.Templates
}

func NewHandler(logger *slog.Logger, templates *htmlutil.Templates) *Handler {
	return &Handler{logger: logger, templates: templates}
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	page := htmlutil.Page{
		MaxUpload: user.GetUserContext(r).MaxUploadBytes(),
		Version:   url.QueryEscape(vcs.Version()),
	}

	err := htmlutil.WriteHTML(w, http.StatusOK, h.templates.App, page)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
