package user

import (
	"context"
	"errors"
	"net/http"
	"server/internal/background"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/session"
	"server/internal/token"
	"server/internal/validator"
)

var ErrEmailTaken = errors.New("email belongs to a verified account")

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
		h.logger.Warn("signup throttled", "key", key)
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

	err := h.store.Save(r.Context(), &u)
	if errors.Is(err, ErrDuplicateEmail) {
		err = h.takeOverPendingSignup(r.Context(), &u)

		switch {
		case errors.Is(err, ErrEmailTaken):
			h.sendExistingAccount(form.Email)

			form.FieldErrors = map[string]string{"email": MsgEmailTaken}
			form.EmailTaken = true
			htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Signup, NewPage(r, form), h.logger)
			return

		case errors.Is(err, ErrEditConflict):
			form.FieldErrors = map[string]string{"email": MsgSignupRetry}
			htmlutil.WriteHTML(w, r, http.StatusConflict, h.templates.Signup, NewPage(r, form), h.logger)
			return
		}
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

func (h *Handler) startVerification(w http.ResponseWriter, r *http.Request, sess *session.Session, u *User) {
	if err := h.sessions.Renew(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.UserID = &u.ID
	sess.Put("pending_email", u.Email)
	sess.Put("flash", MsgCodeSent(u.Email))

	if err := h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/verify", http.StatusSeeOther)
}

func (h *Handler) takeOverPendingSignup(ctx context.Context, u *User) error {
	existing, err := h.store.GetByEmail(ctx, u.Email)
	if err != nil {
		return err
	}

	if existing.Verified {
		return ErrEmailTaken
	}

	u.ID = existing.ID
	u.Version = existing.Version
	u.Verified = false

	if err = h.store.Update(ctx, u); err != nil {
		return err
	}

	if err = h.token.DeleteVerificationsByUserID(ctx, u.ID); err != nil {
		return err
	}

	return h.sessions.DeleteByUserID(ctx, u.ID)
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
