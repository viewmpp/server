package user

import (
	"context"
	"errors"
	"net/http"
	"server/internal/htmlutil"
	"server/internal/safelog"
	"server/internal/session"
	"server/internal/token"
	"strconv"
	"time"
)

type VerifyForm struct {
	Email       string
	FieldErrors map[string]string
}

func (h *Handler) VerifyPage(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)
	h.writeVerify(w, r, http.StatusOK, VerifyForm{Email: sess.Get("pending_email")})
}

func (h *Handler) writeVerify(w http.ResponseWriter, r *http.Request, status int, form VerifyForm) {
	htmlutil.WriteHTML(w, r, status, h.templates.Verify, NewPage(r, form), h.logger)
}

const cooldownKey = "code_until"

func markCooldown(sess *session.Session, until time.Time) {
	sess.Put(cooldownKey, strconv.FormatInt(until.Unix(), 10))
}

func cooldownLeft(r *http.Request) int {
	stamp, err := strconv.ParseInt(session.FromContext(r).Get(cooldownKey), 10, 64)
	if err != nil {
		return 0
	}

	left := time.Until(time.Unix(stamp, 0))
	if left <= 0 {
		return 0
	}

	return int(left.Seconds()) + 1
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	form := VerifyForm{Email: sess.Get("pending_email")}
	code := NormalizeCode(r.PostFormValue("code"))

	ipKey := VisitorKey(r, "verify:")
	if !h.limiter.Allow(ipKey) {
		h.logger.Warn("verify throttled", "key", safelog.Key(ipKey))
		form.FieldErrors = map[string]string{"code": MsgTooManyTries}
		h.writeVerify(w, r, http.StatusTooManyRequests, form)
		return
	}

	if code == "" {
		form.FieldErrors = map[string]string{"code": MsgCodeRequired}
		h.writeVerify(w, r, http.StatusUnprocessableEntity, form)
		return
	}

	u, err := h.store.GetByToken(r.Context(), code, token.ScopeVerification)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			h.limiter.Count(ipKey)
			form.FieldErrors = map[string]string{"code": MsgCodeInvalid}
			h.writeVerify(w, r, http.StatusUnprocessableEntity, form)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	h.limiter.Reset(ipKey)

	u.Verified = true

	if err = h.store.Update(r.Context(), u); err != nil {
		if errors.Is(err, ErrEditConflict) {
			form.FieldErrors = map[string]string{"code": MsgVerifyRetry}
			h.writeVerify(w, r, http.StatusConflict, form)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.token.DeleteVerificationsByUserID(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.Renew(r.Context(), w, sess, &u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}
	sess.Pop("pending_email")
	sess.Put("flash", MsgEmailConfirmed)

	if err = h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ResendCode(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	email := sess.Get("pending_email")
	if email == "" {
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	key := VisitorKey(r, "resend:")
	if !h.limiter.Take(key) {
		h.logger.Warn("resend throttled", "key", safelog.Key(key))

		form := VerifyForm{Email: email, FieldErrors: map[string]string{"code": MsgTooManyTries}}
		h.writeVerify(w, r, http.StatusTooManyRequests, form)
		return
	}

	sent, err := h.resend(r.Context(), email)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if sent {
		markCooldown(sess, time.Now().Add(h.verificationRC))
		sess.Put("flash", MsgCodeUpdated)
	}

	if err := h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/verify", http.StatusSeeOther)
}

func (h *Handler) resend(ctx context.Context, email string) (bool, error) {
	u, err := h.store.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return false, nil
		}
		return false, err
	}

	if u.Verified {
		return false, nil
	}

	issued, exists, err := h.token.LatestVerificationIssuedAt(ctx, u.ID, h.verificationTTL)
	if err != nil {
		return false, err
	}

	if exists && time.Since(issued) < h.verificationRC {
		h.logger.Info("resend throttled", "user_id", u.ID)
		return false, nil
	}

	vry, err := token.NewVerification(u.ID, h.verificationTTL)
	if err != nil {
		return false, err
	}

	if err = h.token.CreateVerification(ctx, vry); err != nil {
		return false, err
	}

	h.sendVerificationCode(u.Email, vry.Plaintext)

	return true, nil
}
