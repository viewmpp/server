package project

import (
	"net/http"
	"server/internal/htmlutil"
	"server/internal/user"
)

type UnlockForm struct {
	FileName    string
	FieldErrors map[string]string
}

func (h *Handler) unlockPage(w http.ResponseWriter, r *http.Request, p *Project, fieldError string) {
	form := UnlockForm{FileName: p.FileName}
	if fieldError != "" {
		form.FieldErrors = map[string]string{"password": fieldError}
	}

	page := user.NewPage(r, form)
	page.NoIndex = true
	page.ProjectID = p.PublicID

	htmlutil.WriteHTML(w, r, http.StatusUnauthorized, h.templates.Unlock, page, h.logger)
}

func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	publicID := r.PathValue("id")

	p, err := h.store.GetByPublicID(r.Context(), publicID)
	if err != nil || p == nil || !p.IsProtected() {
		htmlutil.NotFoundPage(w, r, h.logger)
		return
	}

	keys := []string{
		"unlock:" + publicID,
		"unlock-ip:" + h.limiter.ClientIP(r),
	}

	if key, allowed := h.limiter.AllowAll(keys); !allowed {
		h.logger.Warn("unlock throttled", "key", key)
		h.unlockPage(w, r, p, MsgTooManyTries)
		return
	}

	if !p.PasswordMatches(r.PostFormValue("password")) {
		h.limiter.FailAll(keys)
		h.unlockPage(w, r, p, MsgWrongPassword)
		return
	}

	h.limiter.ResetAll(keys)

	sess.Put(unlockKey(publicID), "1")

	http.Redirect(w, r, "/p/"+publicID, http.StatusSeeOther)
}
