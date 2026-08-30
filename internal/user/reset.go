package user

import (
	"errors"
	"net/http"
	"server/internal/background"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/safelog"
	"server/internal/token"
	"server/internal/validator"
)

type ForgotForm struct {
	Email       string
	FieldErrors map[string]string
}

type ResetForm struct {
	Token       string
	FieldErrors map[string]string
}

func (h *Handler) ForgotPage(w http.ResponseWriter, r *http.Request) {
	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Forgot, NewPage(r, ForgotForm{}), h.logger)
}

func (h *Handler) Forgot(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	form := ForgotForm{Email: NormalizeEmail(r.PostFormValue("email"))}

	v := validator.New()
	CheckEmail(v, "email", form.Email)

	if !v.Valid() {
		form.FieldErrors = v.Errors
		htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Forgot, NewPage(r, form), h.logger)
		return
	}

	keys := []string{"reset:" + form.Email, "reset-ip:" + clientip.From(r)}

	if key, allowed := h.limiter.TakeAll(keys); !allowed {
		h.logger.Warn("reset throttled", "limit", safelog.Key(key))
		form.FieldErrors = map[string]string{"email": MsgTooManyTries}
		htmlutil.WriteHTML(w, r, http.StatusTooManyRequests, h.templates.Forgot, NewPage(r, form), h.logger)
		return
	}

	if err := h.issueReset(r, form.Email); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.Put("flash", MsgResetSent(form.Email))

	http.Redirect(w, r, "/reset", http.StatusSeeOther)
}

func (h *Handler) issueReset(r *http.Request, email string) error {
	u, err := h.store.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		return err
	}

	if !u.Verified {
		h.logger.Info("reset skipped for unverified account", "user_id", u.ID)
		return nil
	}

	rst, err := token.NewReset(u.ID, h.resetTTL)
	if err != nil {
		return err
	}

	if err = h.token.CreateReset(r.Context(), rst); err != nil {
		return err
	}

	link := h.baseURL + "/reset/" + rst.Plaintext

	background.Run(h.wg, h.logger, func() {
		if err := h.mailer.SendPasswordReset(u.Email, link); err != nil {
			h.logger.Error("failed to send password reset email", "err", err)
		}
	})

	return nil
}

func (h *Handler) ResetPage(w http.ResponseWriter, r *http.Request) {
	plaintext := r.PathValue("token")

	if _, err := h.store.GetByToken(r.Context(), plaintext, token.ScopeReset); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Reset,
		NewPage(r, ResetForm{Token: plaintext}), h.logger)
}

func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	plaintext := r.PathValue("token")
	form := ResetForm{Token: plaintext}

	pass := r.PostFormValue("password")

	v := validator.New()
	CheckPassword(v, "password", pass)

	if !v.Valid() {
		form.FieldErrors = v.Errors
		htmlutil.WriteHTML(w, r, http.StatusUnprocessableEntity, h.templates.Reset, NewPage(r, form), h.logger)
		return
	}

	u, err := h.store.GetByToken(r.Context(), plaintext, token.ScopeReset)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = u.password.Set(pass); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.store.Update(r.Context(), u); err != nil {
		if errors.Is(err, ErrEditConflict) {
			form.FieldErrors = map[string]string{"password": MsgVerifyRetry}
			htmlutil.WriteHTML(w, r, http.StatusConflict, h.templates.Reset, NewPage(r, form), h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.token.DeleteResetsByUserID(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.DeleteByUserID(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.Renew(r.Context(), w, sess, &u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	sess.Put("flash", MsgPasswordChanged)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
