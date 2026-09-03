package diagnostics

import (
	"expvar"
	"net/http"
)

func route() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /debug/vars", expvar.Handler())

	return mux
}
