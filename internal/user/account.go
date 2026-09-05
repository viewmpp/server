package user

import (
	"net/http"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/safelog"
	"server/internal/session"
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

	saved, err := h.projects.CountByUserID(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	shared, err := h.projects.CountShared(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	form := AccountForm{}

	sess := session.FromContext(r)
	if field := sess.Pop(accountErrorField); field != "" {
		form.FieldErrors = map[string]string{field: sess.Pop(accountErrorMessage)}
	}

	page := NewPage(r, form)
	page.NoIndex = true
	page.SavedCount = saved
	page.SharedCount = shared

	if u.SubscriptionUntil != nil {
		page.ProUntil = u.SubscriptionUntil.Format("2 Jan 2006")
	}

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
		h.logger.Warn("password change throttled", "limit", safelog.Key(key))
		h.accountError(w, r, "current", MsgTooManyTries)
		return
	}

	current := r.PostFormValue("current")
	next := r.PostFormValue("password")

	v := validator.New()
	CheckPassword(v, "password", next)

	if !v.Valid() {
		h.accountError(w, r, "password", v.Errors["password"])
		return
	}

	matches, err := u.password.Matches(current)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if !matches {
		h.limiter.CountAll(keys)
		h.accountError(w, r, "current", MsgWrongCurrentPassword)
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

	if err = h.sessions.Renew(r.Context(), w, sess, nil); err != nil {
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
		h.logger.Warn("account deletion throttled", "limit", safelog.Key(key))
		h.accountError(w, r, "confirm", MsgTooManyTries)
		return
	}

	matches, err := u.password.Matches(r.PostFormValue("confirm"))
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if !matches {
		h.limiter.CountAll(keys)
		h.accountError(w, r, "confirm", MsgWrongCurrentPassword)
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

const (
	accountErrorField   = "account_error_field"
	accountErrorMessage = "account_error_message"
)

func (h *Handler) accountError(w http.ResponseWriter, r *http.Request, field, message string) {
	sess := session.FromContext(r)
	sess.Put(accountErrorField, field)
	sess.Put(accountErrorMessage, message)

	http.Redirect(w, r, "/account", http.StatusSeeOther)
}
