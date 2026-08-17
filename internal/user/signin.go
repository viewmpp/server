package user

import (
	"errors"
	"net/http"
	"server/internal/clientip"
	"server/internal/htmlutil"

	"golang.org/x/crypto/bcrypt"
)

type SigninForm struct {
	Email       string
	FieldErrors map[string]string
}

var dummyHash = func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)
	return h
}()

func (h *Handler) SigninPage(w http.ResponseWriter, r *http.Request) {
	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Signin, NewPage(r, SigninForm{}), h.logger)
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	form := SigninForm{Email: NormalizeEmail(r.PostFormValue("email"))}
	pass := r.PostFormValue("password")

	keys := []string{"signin:" + form.Email, "signin-ip:" + clientip.From(r)}

	if key, ok := h.limiter.AllowAll(keys); !ok {
		h.logger.Warn("signin throttled", "key", key)
		form.FieldErrors = map[string]string{"email": MsgTooManyTries}
		htmlutil.WriteHTML(w, r, http.StatusTooManyRequests, h.templates.Signin, NewPage(r, form), h.logger)
		return
	}

	u, err := h.store.GetByEmail(r.Context(), form.Email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	var matched bool
	if u == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(pass))
	} else {
		matched, err = u.password.Matches(pass)
		if err != nil {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
	}

	if !matched {
		h.limiter.CountAll(keys)

		form.FieldErrors = map[string]string{"email": MsgEmailOrPass}
		htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Signin, NewPage(r, form), h.logger)
		return
	}

	h.limiter.ResetAll(keys)

	if err = h.sessions.Renew(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.UserID = &u.ID

	if !u.Verified {
		sess.Put("pending_email", u.Email)
		sess.Put("flash", MsgVerifyLater)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Signout(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	if err := h.sessions.Clear(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
