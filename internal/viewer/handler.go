package viewer

import (
	"html/template"
	"log/slog"
	"mpp-viewer-server/internal/jsonutil"
	"net/http"
)

type Handler struct {
	logger *slog.Logger
}

func NewHandler(logger *slog.Logger) *Handler {
	return &Handler{
		logger: logger,
	}
}

func (h *Handler) View(w http.ResponseWriter, r *http.Request) {
	ts, err := template.ParseFiles("./ui/html/viewer.tmpl")
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	err = ts.Execute(w, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
