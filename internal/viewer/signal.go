package viewer

import (
	"net/http"
	"server/internal/events"
	"server/internal/jsonutil"
)

func (h *Handler) Signal(w http.ResponseWriter, r *http.Request) {
	name, ok := events.FromViewer(r.URL.Query().Get("name"))
	if !ok {
		jsonutil.NotFoundResponse(w)
		return
	}

	h.counts.Count(name)

	w.WriteHeader(http.StatusNoContent)
}
