package user

import (
	"context"
	"errors"
	"net/http"
	"server/internal/background"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/safelog"
	"server/internal/session"
	"server/internal/token"
	"server/internal/validator"
)

var ErrEmailTaken = errors.New("email belongs to an existing account")

type signupStore interface {
	Save(ctx context.Context, user *User) error
}

type SignupForm struct {
	Email       string
	FieldErrors map[string]string
	EmailTaken  bool
}

func (h *Handler) SignupPage(w http.ResponseWriter, r *http.Request) {
	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Signup, NewPage(r, SignupForm{}), h.logger)
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	form := SignupForm{Email: NormalizeEmail(r.PostFormValue("email"))}
	pass := r.PostFormValue("password")

	keys := []string{"signup:" + form.Email, "signup-ip:" + clientip.From(r)}

	if key, ok := h.limiter.TakeAll(keys); !ok {
		h.logger.Warn("signup throttled", "limit", safelog.Key(key))
		form.FieldErrors = map[string]string{"email": MsgTooManyTries}
		htmlutil.WriteHTML(w, r, http.StatusTooManyRequests, h.templates.Signup, NewPage(r, form), h.logger)
		return
	}

	v := validator.New()
	CheckEmail(v, "email", form.Email)
	CheckPassword(v, "password", pass)

	if !v.Valid() {
		form.FieldErrors = v.Errors
		htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Signup, NewPage(r, form), h.logger)
		return
	}

	u := User{Email: form.Email}
	if err := u.password.Set(pass); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	err := createSignup(r.Context(), h.store, &u)
	if errors.Is(err, ErrEmailTaken) {
		h.sendExistingAccount(form.Email)

		form.FieldErrors = map[string]string{"email": MsgEmailTaken}
		form.EmailTaken = true
		htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Signup, NewPage(r, form), h.logger)
		return
	}

	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	vry, err := token.NewVerification(u.ID, h.verificationTTL)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.token.CreateVerification(r.Context(), vry); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.sendVerificationCode(u.Email, vry.Plaintext)

	h.startVerification(w, r, sess, &u)
}

func createSignup(ctx context.Context, store signupStore, u *User) error {
	err := store.Save(ctx, u)
	if errors.Is(err, ErrDuplicateEmail) {
		return ErrEmailTaken
	}
	return err
}

func (h *Handler) startVerification(w http.ResponseWriter, r *http.Request, sess *session.Session, u *User) {
	if err := h.sessions.Renew(r.Context(), w, sess, &u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	sess.Put("pending_email", u.Email)

	if err := h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/verify", http.StatusSeeOther)
}

func (h *Handler) sendVerificationCode(email, code string) {
	background.Run(h.wg, h.logger, func() {
		if err := h.mailer.SendVerification(email, code); err != nil {
			h.logger.Error("failed to send verification email", "err", err)
		}
	})
}

func (h *Handler) sendExistingAccount(email string) {
	background.Run(h.wg, h.logger, func() {
		if err := h.mailer.SendExistingAccount(email); err != nil {
			h.logger.Error("failed to send existing account email", "err", err)
		}
	})
}
