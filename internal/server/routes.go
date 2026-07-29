package server

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/healthcheck", s.healthcheck)

	mux.HandleFunc("/v1/home", s.viewerHandler.View)

	return mux
}
