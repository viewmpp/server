package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"server/ui"
)

var errorPage = template.Must(template.ParseFS(ui.Files, "templates/error.tmpl"))

func errorResponse(w http.ResponseWriter, status int) {
	buf := new(bytes.Buffer)
	err := errorPage.Execute(buf, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	logger.Error("page render failed", "err", err, "method", r.Method, "uri", r.URL.Path)
	errorResponse(w, http.StatusInternalServerError)
}

func BadRequestPage(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	logger.Warn("request rejected", "method", r.Method, "path", r.URL.Path)
	errorResponse(w, http.StatusBadRequest)
}

func NotFoundPage(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	logger.Info("not found", "method", r.Method, "path", r.URL.Path)
	errorResponse(w, http.StatusNotFound)
}
