package debug

import (
	"expvar"
	"net/http"
)

func (s *Server) route() http.Handler {

	mux := http.NewServeMux()

	mux.Handle("GET /debug/vars", expvar.Handler())

	return mux
}
