package viewer

import (
	"log/slog"
	"net/http"
	"server/internal/htmlutil"
	"server/internal/user"
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
	htmlutil.Render(w, r, http.StatusOK, h.templates.App, user.NewPage(r, nil), h.logger)
}
