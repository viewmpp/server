package user

import (
	"errors"
	"net/http"
	"server/internal/htmlutil"
	"server/internal/ratelimit"
	"server/internal/session"

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
	htmlutil.Render(w, r, http.StatusOK, h.templates.Signin, NewPage(r, SigninForm{}), h.logger)
}

func (h *Handler) Signin(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	form := SigninForm{Email: NormalizeEmail(r.PostFormValue("email"))}
	pass := r.PostFormValue("password")

	keys := []string{"signin:" + form.Email, "signin-ip:" + ratelimit.ClientIP(r)}

	for _, key := range keys {
		if !h.limiter.Allow(key) {
			h.logger.Warn("signin throttled", "key", key)
			form.FieldErrors = map[string]string{"email": MsgTooManyTries}
			htmlutil.Render(w, r, http.StatusTooManyRequests, h.templates.Signin, NewPage(r, form), h.logger)
			return
		}
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
		for _, key := range keys {
			h.limiter.Fail(key)
		}

		form.FieldErrors = map[string]string{"email": MsgEmailOrPass}
		htmlutil.Render(w, r, http.StatusUnprocessableEntity, h.templates.Signin, NewPage(r, form), h.logger)
		return
	}

	for _, key := range keys {
		h.limiter.Reset(key)
	}

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
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if err := h.sessions.Clear(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
