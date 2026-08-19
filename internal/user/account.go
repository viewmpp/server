package user

import (
	"net/http"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/validator"
	"strconv"
)

type AccountForm struct {
	FieldErrors map[string]string
}

func (h *Handler) AccountPage(w http.ResponseWriter, r *http.Request) {
	u := GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	page := NewPage(r, AccountForm{})
	page.NoIndex = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Account, page, h.logger)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	keys := []string{
		"password-user:" + strconv.FormatInt(u.ID, 10),
		"password-ip:" + clientip.From(r),
	}

	if key, allowed := h.limiter.AllowAll(keys); !allowed {
		h.logger.Warn("password change throttled", "key", key)
		h.accountError(w, r, "current", MsgTooManyTries, http.StatusTooManyRequests)
		return
	}

	current := r.PostFormValue("current")
	next := r.PostFormValue("password")

	v := validator.New()
	CheckPassword(v, "password", next)

	if !v.Valid() {
		h.accountError(w, r, "password", v.Errors["password"], http.StatusUnprocessableEntity)
		return
	}

	matches, err := u.password.Matches(current)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if !matches {
		h.limiter.CountAll(keys)
		h.accountError(w, r, "current", MsgWrongCurrentPassword, http.StatusUnprocessableEntity)
		return
	}

	h.limiter.ResetAll(keys)

	if err = u.password.Set(next); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.store.Update(r.Context(), u); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.DeleteByUserID(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.Renew(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.Put("flash", MsgPasswordChanged)

	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	keys := []string{
		"delete-user:" + strconv.FormatInt(u.ID, 10),
		"delete-ip:" + clientip.From(r),
	}

	if key, allowed := h.limiter.AllowAll(keys); !allowed {
		h.logger.Warn("account deletion throttled", "key", key)
		h.accountError(w, r, "confirm", MsgTooManyTries, http.StatusTooManyRequests)
		return
	}

	matches, err := u.password.Matches(r.PostFormValue("confirm"))
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if !matches {
		h.limiter.CountAll(keys)
		h.accountError(w, r, "confirm", MsgWrongCurrentPassword, http.StatusUnprocessableEntity)
		return
	}

	if err = h.store.Delete(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.Clear(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) accountError(w http.ResponseWriter, r *http.Request, field, message string, status int) {
	page := NewPage(r, AccountForm{FieldErrors: map[string]string{field: message}})
	page.NoIndex = true

	htmlutil.WriteHTML(w, r, status, h.templates.Account, page, h.logger)
}
