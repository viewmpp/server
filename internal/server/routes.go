package server

import (
	"net/http"
	"server/ui"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.static())

	mux.HandleFunc("GET /{$}", s.viewerHandler.Landing)

	mux.HandleFunc("GET /api/v1/healthcheck", s.healthcheck)
	mux.HandleFunc("POST /api/v1/upload", s.uploadHandler.Upload)

	return s.recoverPanic(s.logRequest(s.authenticate(mux)))
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(ui.Files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Env == "dev" {
			w.Header().Set("Cache-Control", "no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
