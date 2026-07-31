package server

import (
	"net/http"
	"server/ui"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(ui.Files))

	mux.HandleFunc("GET /{$}", s.viewerHandler.Upload)
	mux.HandleFunc("GET /viewer", s.viewerHandler.View)

	mux.HandleFunc("GET /api/v1/healthcheck", s.healthcheck)

	return s.recoverPanic(s.logRequest(mux))
}
