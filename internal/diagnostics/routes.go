package diagnostics

import (
	"expvar"
	"net/http"
)

func routes(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /debug/vars", expvar.Handler())
	mux.HandleFunc("GET /events", h.EventList)

	return mux
}
