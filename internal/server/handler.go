package server

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/safelog"
	"server/internal/vcs"
	"server/ui"
	"strings"
	"time"
)

type healthcheck struct {
	Status  string `json:"status"`
	Parser  string `json:"parser"`
	Env     string `json:"env"`
	Version string `json:"version"`
}

func (s *Server) serverHealthcheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	parserStatus := s.parserHealthcheck(ctx)

	hc := healthcheck{
		Status:  "OK",
		Parser:  parserStatus,
		Env:     s.cfg.AppEnv,
		Version: vcs.Version,
	}

	err := jsonutil.WriteJSON(w, http.StatusOK, hc, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, s.logger)
	}
}

func (s *Server) parserHealthcheck(ctx context.Context) string {
	err := s.client.Healthcheck(ctx)
	if err != nil {
		s.logger.Warn("error while checking parser's healthcheck endpoint", "err", err)
		return "DOWN"
	}

	return "OK"
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(ui.Files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if !strings.HasPrefix(name, "static/") || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		info, err := fs.Stat(ui.Files, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		switch s.cfg.AppEnv {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

const appHeader = "X-Requested-With"

const appHeaderValue = "viewmpp"

func (s *Server) fromApp(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(appHeader) != appHeaderValue {
			s.logger.Warn("api call without the app header", "method", r.Method, "uri", safelog.URI(r.URL.Path))
			jsonutil.ForbiddenResponse(w, "this endpoint is called by the viewer, not directly")
			return
		}

		next(w, r)
	}
}

func (s *Server) icon(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch s.cfg.AppEnv {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}

		http.ServeFileFS(w, r, ui.Files, "static/icons/"+name)
	}
}

func (s *Server) redirect(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusSeeOther)
	}
}

func (s *Server) moved(to string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusMovedPermanently)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonutil.NotFoundResponse(w)
		return
	}

	htmlutil.NotFoundPage(w, r, s.logger)
}
