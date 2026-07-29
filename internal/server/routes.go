package server

import (
	"net/http"
	"server/internal/fixture"
	"server/ui"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(ui.Files))

	mux.HandleFunc("GET /{$}", s.viewerHandler.Upload)
	mux.HandleFunc("GET /viewer", s.viewerHandler.View)

	mux.HandleFunc("GET /api/v1/healthcheck", s.healthcheck)
	mux.HandleFunc("GET /api/v1/schedule", s.schedule)

	return s.recoverPanic(s.logRequest(mux))
}

func (s *Server) schedule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(fixture.ByDemo(r.URL.Query().Get("demo")).Contract)
}
