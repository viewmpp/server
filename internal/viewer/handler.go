package viewer

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"server/internal/fixture"
	"server/ui"
)

const errorPage = `<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><title>Ошибка — MPP Viewer</title></head>
<body><h1>Что-то пошло не так</h1>
<p>Страницу не удалось построить. Попробуйте обновить.</p></body></html>`

type Handler struct {
	logger *slog.Logger
	pages  map[string]*template.Template
}

func NewHandler(logger *slog.Logger) (*Handler, error) {
	pages, err := newTemplateCache()
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Handler{logger: logger, pages: pages}, nil
}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(ui.Files, "html/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		ts, err := template.ParseFS(ui.Files, "html/base.tmpl", page)
		if err != nil {
			return nil, err
		}
		cache[path.Base(page)] = ts
	}

	return cache, nil
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "upload.tmpl", nil)
}

type viewerPage struct {
	FileName string
}

func (h *Handler) View(w http.ResponseWriter, r *http.Request) {
	page := viewerPage{FileName: fixture.ByDemo(r.URL.Query().Get("demo")).FileName}
	h.render(w, r, http.StatusOK, "viewer.tmpl", page)
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	ts, ok := h.pages[page]
	if !ok {
		h.serverError(w, r, fmt.Errorf("template %q does not exist", page))
		return
	}

	buf := new(bytes.Buffer)
	if err := ts.ExecuteTemplate(buf, "base", data); err != nil {
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.Error("page render failed",
		"err", err, "method", r.Method, "uri", r.URL.RequestURI())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = io.WriteString(w, errorPage)
}
