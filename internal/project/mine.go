package project

import (
	"net/http"
	"server/internal/jsonutil"
	"server/internal/user"
	"time"
)

type savedPlan struct {
	ID        string    `json:"id"`
	FileName  string    `json:"file_name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	projects, err := h.store.ListByUserID(r.Context(), u.ID, h.listLimit)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	plans := make([]savedPlan, 0, len(projects))
	for _, p := range projects {
		plans = append(plans, savedPlan{ID: p.PublicID, FileName: p.FileName, CreatedAt: p.CreatedAt})
	}

	if err = jsonutil.WriteJSON(w, http.StatusOK, plans, nil); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
