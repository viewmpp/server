package diagnostics

import (
	"context"
	"log/slog"
	"net/http"
	"server/internal/events"
	"server/internal/jsonutil"
	"time"
)

const eventWindow = 30 * 24 * time.Hour

type totals interface {
	Totals(ctx context.Context, since time.Time) ([]events.Total, error)
}

type Handler struct {
	events totals
	logger *slog.Logger
}

func NewHandler(events totals, logger *slog.Logger) *Handler {
	return &Handler{
		events: events,
		logger: logger,
	}
}

func (h *Handler) EventList(w http.ResponseWriter, r *http.Request) {
	list, err := h.events.Totals(r.Context(), time.Now().Add(-eventWindow))
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = jsonutil.WriteJSON(w, http.StatusOK, list, nil); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
