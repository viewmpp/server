package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"server/internal/jsonutil"
	"server/ui"
)

var errorPage = template.Must(template.ParseFS(ui.Files, "html/error.tmpl"))

func errorResponse(w http.ResponseWriter, r *http.Request, status int, logger *slog.Logger) {
	buf := new(bytes.Buffer)
	err := errorPage.Execute(buf, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, logger)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	logger.Error("page render failed", "err", err, "method", r.Method, "uri", r.URL.RequestURI())
	errorResponse(w, r, http.StatusInternalServerError, logger)
}
