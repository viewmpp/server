package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"server/internal/safelog"
	"server/ui"
)

var serverError = template.Must(template.ParseFS(ui.Files, "templates/errors/server_error.tmpl"))

var badRequest = template.Must(template.ParseFS(ui.Files, "templates/errors/bad_request.tmpl"))

var notFound = template.Must(template.ParseFS(ui.Files, "templates/errors/not_found.tmpl"))

var tooMany = template.Must(template.ParseFS(ui.Files, "templates/errors/too_many.tmpl"))

var invalidToken = template.Must(template.ParseFS(ui.Files, "templates/errors/invalid_token.tmpl"))

func errorResponse(w http.ResponseWriter, status int, tmpl *template.Template) {
	buf := new(bytes.Buffer)

	err := tmpl.Execute(buf, nil)
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
	logger.Error("page render failed", "err", err, "method", r.Method, "uri", safelog.URI(r.URL.Path))
	errorResponse(w, http.StatusInternalServerError, serverError)
}

func BadRequestPage(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	logger.Warn("request rejected", "method", r.Method, "path", safelog.URI(r.URL.Path))
	errorResponse(w, http.StatusBadRequest, badRequest)
}

func TooManyRequestsPage(w http.ResponseWriter) {
	errorResponse(w, http.StatusTooManyRequests, tooMany)
}

func NotFoundPage(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	logger.Info("not found", "method", r.Method, "path", safelog.URI(r.URL.Path))
	errorResponse(w, http.StatusNotFound, notFound)
}

func InvalidResetTokenPage(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	logger.Warn("invalid or expired token", "method", r.Method, "path", safelog.URI(r.URL.Path))
	errorResponse(w, http.StatusBadRequest, invalidToken)
}
