package viewer

import (
	"log/slog"
	"net/http"
	"server/internal/htmlutil"
)

type Handler struct {
	logger    *slog.Logger
	templates *htmlutil.Templates
}

func NewHandler(logger *slog.Logger, templates *htmlutil.Templates) *Handler {
	return &Handler{logger: logger, templates: templates}
}

func (h *Handler) Landing(w http.ResponseWriter, r *http.Request) {
	page := htmlutil.Page{}
	err := htmlutil.WriteHTML(w, http.StatusOK, h.templates.Upload, page)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}

func (h *Handler) View(w http.ResponseWriter, r *http.Request) {
	page := htmlutil.Page{}
	err := htmlutil.WriteHTML(w, http.StatusOK, h.templates.Viewer, page)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
