package server

import (
	"net/http"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/vcs"
	"server/ui"
	"strings"
)

type healthcheck struct {
	Status  string `json:"status"`
	Env     string `json:"env"`
	Version string `json:"version"`
}

func (s *Server) healthcheck(w http.ResponseWriter, r *http.Request) {
	hc := healthcheck{
		Status:  "OK",
		Env:     s.cfg.Env,
		Version: vcs.Version(),
	}

	err := jsonutil.WriteJSON(w, http.StatusOK, hc, nil)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, s.logger)
	}
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(ui.Files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch s.cfg.Env {
		case "dev":
			w.Header().Set("Cache-Control", "no-store")
		default:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) icon(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800")

		http.ServeFileFS(w, r, ui.Files, "static/icons/"+name)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonutil.NotFoundResponse(w)
		return
	}

	htmlutil.NotFoundPage(w, r, s.logger)
}
