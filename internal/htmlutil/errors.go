package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"server/ui"
)

var serverError = template.Must(template.ParseFS(ui.Files, "templates/errors/server_error.tmpl"))

var badRequest = template.Must(template.ParseFS(ui.Files, "templates/errors/bad_request.tmpl"))

var notFound = template.Must(template.ParseFS(ui.Files, "templates/errors/not_found.tmpl"))

func errorResponse(w http.ResponseWriter, status int) {
	buf := new(bytes.Buffer)
	var err error
	switch status {
	case http.StatusBadRequest:
		err = badRequest.Execute(buf, nil)
	case http.StatusNotFound:
		err = notFound.Execute(buf, nil)
	default:
		err = serverError.Execute(buf, nil)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
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
