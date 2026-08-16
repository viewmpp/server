package project

import (
	"io"
	"net/http"
	"server/internal/contract"
	"server/internal/export"
	"server/internal/jsonutil"
	"server/internal/user"
)

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		jsonutil.UnauthorizedResponse(w)
		return
	}

	if r.ContentLength > export.MaxContractBytes {
		jsonutil.ContentTooLargeError(w)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, export.MaxContractBytes))
	if err != nil {
		jsonutil.BadRequestResponse(w, "plan is too large or could not be read")
		return
	}

	if _, err = contract.Decode(body); err != nil {
		h.logger.Warn("save rejected", "err", err)
		jsonutil.BadRequestResponse(w, "that is not a valid plan")
		return
	}

	publicID, err := h.store.Save(r.Context(), u.ID, sanitizeFileName(r.URL.Query().Get("name")), body)
	if err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	w.Header().Set("Location", "/p/"+publicID)

	if err = jsonutil.WriteJSON(w, http.StatusCreated, map[string]string{"id": publicID}, nil); err != nil {
		jsonutil.ServerErrorResponse(w, r, err, h.logger)
	}
}
