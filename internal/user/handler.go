package user

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"server/internal/background"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/ratelimit"
	"server/internal/session"
	"server/internal/token"
	"server/internal/validator"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store           *Store
	token           *token.Store
	sessions        *session.Store
	limiter         *ratelimit.Limiter
	mailer          *mailer.Mailer
	verificationTTL time.Duration
	verificationRC  time.Duration
	templates       *htmlutil.Templates
	wg              *sync.WaitGroup
	logger          *slog.Logger
}

func NewHandler(
	store *Store,
	token *token.Store,
	sessions *session.Store,
	limiter *ratelimit.Limiter,
	mailer *mailer.Mailer,
	vttl time.Duration,
	vrc time.Duration,
	templates *htmlutil.Templates,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:           store,
		token:           token,
		sessions:        sessions,
		limiter:         limiter,
		mailer:          mailer,
		verificationTTL: vttl,
		verificationRC:  vrc,
		templates:       templates,
		wg:              wg,
		logger:          logger,
	}
}

var ErrEmailTaken = errors.New("email belongs to a verified account")

type SignupForm struct {
	Email       string
	FieldErrors map[string]string
	EmailTaken  bool
}

func (h *Handler) SignupPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, h.templates.Signup, SignupForm{}, h.logger)
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	form := SignupForm{Email: NormalizeEmail(r.PostFormValue("email"))}
	pass := r.PostFormValue("password")

	v := validator.New()
	CheckEmail(v, "email", form.Email)
	CheckPassword(v, "password", pass)

	if !v.Valid() {
		form.FieldErrors = v.Errors
		render(w, r, http.StatusUnprocessableEntity, h.templates.Signup, form, h.logger)
		return
	}

	u := User{Email: form.Email}
	if err := u.password.Set(pass); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	err := h.store.Create(r.Context(), &u)
	if errors.Is(err, ErrDuplicateEmail) {
		err = h.takeOverPendingSignup(r.Context(), &u)

		switch {
		case errors.Is(err, ErrEmailTaken):
			form.FieldErrors = map[string]string{"email": MsgEmailTaken}
			form.EmailTaken = true
			render(w, r, http.StatusUnprocessableEntity, h.templates.Signup, form, h.logger)
			return

		case errors.Is(err, ErrEditConflict):
			form.FieldErrors = map[string]string{"email": MsgSignupRetry}
			render(w, r, http.StatusConflict, h.templates.Signup, form, h.logger)
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

	if err = h.sessions.Renew(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.UserID = &u.ID
	sess.Put("pending_email", form.Email)
	sess.Put("flash", MsgCodeSent(form.Email))

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

func (h *Handler) sendExistingAccount(user User) {
	background.Run(h.wg, h.logger, func() {
		err := h.mailer.SendExistingAccount(user.Email)
		if err != nil {
			h.logger.Error("failed to send verification email", "err", err)
			return
		}
	})
}

type VerifyForm struct {
	Email       string
	FieldErrors map[string]string
}

func (h *Handler) VerifyPage(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)
	render(w, r, http.StatusOK, h.templates.Verify, VerifyForm{Email: sess.Get("pending_email")}, h.logger)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	form := VerifyForm{Email: sess.Get("pending_email")}
	code := NormalizeCode(r.PostFormValue("code"))

	ipKey := "verify-ip:" + ratelimit.ClientIP(r)
	if !h.limiter.Allow(ipKey) {
		h.logger.Warn("verify throttled", "key", ipKey)
		form.FieldErrors = map[string]string{"code": MsgTooManyTries}
		render(w, r, http.StatusTooManyRequests, h.templates.Verify, form, h.logger)
		return
	}

	if code == "" {
		form.FieldErrors = map[string]string{"code": MsgCodeRequired}
		render(w, r, http.StatusUnprocessableEntity, h.templates.Verify, form, h.logger)
		return
	}

	u, err := h.store.GetByToken(r.Context(), code, token.ScopeVerification)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			h.limiter.Fail(ipKey)
			form.FieldErrors = map[string]string{"code": MsgCodeInvalid}
			render(w, r, http.StatusUnprocessableEntity, h.templates.Verify, form, h.logger)
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
			render(w, r, http.StatusConflict, h.templates.Verify, form, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.token.DeleteVerificationsByUserID(r.Context(), u.ID); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	if err = h.sessions.Renew(r.Context(), w, sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.UserID = &u.ID
	sess.Pop("pending_email")
	sess.Put("flash", MsgEmailConfirmed)

	if err = h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ResendCode(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	email := sess.Get("pending_email")
	if email == "" {
		http.Redirect(w, r, "/signup", http.StatusSeeOther)
		return
	}

	if err := h.resend(r.Context(), email); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.Put("flash", MsgCodeResent(email))

	if err := h.sessions.Save(r.Context(), sess); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	http.Redirect(w, r, "/verify", http.StatusSeeOther)
}

func (h *Handler) resend(ctx context.Context, email string) error {
	u, err := h.store.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		return err
	}

	if u.Verified {
		return nil
	}

	issued, exists, err := h.token.LatestVerificationIssuedAt(ctx, u.ID, h.verificationTTL)
	if err != nil {
		return err
	}

	if exists && time.Since(issued) < h.verificationRC {
		h.logger.Info("resend throttled", "user_id", u.ID)
		return nil
	}

	vry, err := token.NewVerification(u.ID, h.verificationTTL)
	if err != nil {
		return err
	}

	if err = h.token.CreateVerification(ctx, vry); err != nil {
		return err
	}

	h.sendVerificationCode(u.Email, vry.Plaintext)

	return nil
}

type LoginForm struct {
	Email       string
	FieldErrors map[string]string
}

var dummyHash = func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)
	return h
}()

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, h.templates.Login, LoginForm{}, h.logger)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	if !session.VerifyCSRF(sess, r) {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	form := LoginForm{Email: NormalizeEmail(r.PostFormValue("email"))}
	pass := r.PostFormValue("password")

	keys := []string{"login:" + form.Email, "login-ip:" + ratelimit.ClientIP(r)}

	for _, key := range keys {
		if !h.limiter.Allow(key) {
			h.logger.Warn("login throttled", "key", key)
			form.FieldErrors = map[string]string{"email": MsgTooManyTries}
			render(w, r, http.StatusTooManyRequests, h.templates.Login, form, h.logger)
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
		render(w, r, http.StatusUnprocessableEntity, h.templates.Login, form, h.logger)
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

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
