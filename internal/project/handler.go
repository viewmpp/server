package project

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"server/internal/contract"
	"server/internal/fixtures"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/landing"
	"server/internal/ratelimit"
	"server/internal/user"
	"server/internal/xlsx"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store     *Store
	baseURL   string
	limiter   *ratelimit.Limiter
	listLimit int
	templates *htmlutil.Templates
	logger    *slog.Logger
}

func NewHandler(
	store *Store,
	baseURL string,
	limiter *ratelimit.Limiter,
	listLimit int,
	templates *htmlutil.Templates,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		store:     store,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		limiter:   limiter,
		listLimit: listLimit,
		templates: templates,
		logger:    logger,
	}
}

func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, false)
	if !ok {
		return
	}

	u := user.GetUserContext(r)

	shared := 0
	if !u.IsAnonymous() {
		var err error
		if shared, err = h.store.CountShared(r.Context(), u.ID); err != nil {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
	}

	page := user.NewPage(r, nil)
	page.NoIndex = true
	page.Description = fmt.Sprintf("%s - an MS Project plan you can open in the browser, no install needed.", p.FileName)
	page.ProjectID = p.PublicID
	page.FileName = p.FileName
	page.Access = p.Access
	page.IsOwner = !u.IsAnonymous() && u.ID == p.UserID
	page.CanShare = u.CanShare(shared)
	page.CanProtect = u.CanProtect()
	page.MaxPublicFree = user.MaxPublicFree
	page.MinPasswordLength = MinPasswordLength
	page.MaxPasswordLength = MaxPasswordLength

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.App, page, h.logger)
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, false)
	if !ok {
		return
	}

	c, err := contract.Decode(p.Contract)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	var buf bytes.Buffer
	if err = xlsx.Write(&buf, c); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", xlsx.Disposition(p.FileName))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	_, _ = buf.WriteTo(w)
}

func (h *Handler) Contract(w http.ResponseWriter, r *http.Request) {
	p, ok := h.find(w, r, true)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(p.Contract)
}

func (h *Handler) SetAccess(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	publicID := r.PathValue("id")

	access := r.PostFormValue("access")
	if access != AccessPublic && access != AccessPrivate && access != AccessProtected {
		htmlutil.BadRequestPage(w, r, h.logger)
		return
	}

	back := fmt.Sprintf("/p/%s", publicID)

	current, err := h.store.GetAccess(r.Context(), publicID, u.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	var password []byte

	if access == AccessProtected && !u.CanProtect() {
		sess.Put("flash", protectRefusal(u))
		http.Redirect(w, r, back, http.StatusSeeOther)
		return
	}

	keepPassword := false

	if access == AccessProtected {
		plaintext := r.PostFormValue("password")

		if plaintext == "" && current == AccessProtected {
			keepPassword = true
		} else {
			if n := utf8.RuneCountInString(plaintext); n < MinPasswordLength || n > MaxPasswordLength {
				sess.Put("flash", MsgPasswordLength(MinPasswordLength))
				http.Redirect(w, r, back, http.StatusSeeOther)
				return
			}

			hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
			if err != nil {
				htmlutil.ServerErrorResponse(w, r, err, h.logger)
				return
			}
			password = hash
		}
	}

	save := func() error {
		if keepPassword {
			return h.store.SetAccessKeepingPassword(r.Context(), publicID, u.ID, access)
		}
		return h.store.SetAccess(r.Context(), publicID, u.ID, access, password)
	}

	if err := save(); err != nil {
		if errors.Is(err, user.ErrShareLimit) {
			var quota *user.QuotaError
			if errors.As(err, &quota) {
				sess.Put("flash", MsgShareLimit(quota.Limit))
				http.Redirect(w, r, back, http.StatusSeeOther)
				return
			}
		}
		if errors.Is(err, user.ErrShareUnverified) {
			sess.Put("flash", MsgConfirmEmail)
			http.Redirect(w, r, back, http.StatusSeeOther)
			return
		}
		if errors.Is(err, ErrNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	switch access {
	case AccessPublic:
		sess.Put("flash", MsgNowPublic)
	case AccessProtected:
		sess.Put("flash", MsgNowProtected)
	default:
		sess.Put("flash", MsgNowPrivate)
	}

	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess, ok := htmlutil.AcceptPost(w, r, h.logger)
	if !ok {
		return
	}

	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	if err := h.store.Delete(r.Context(), r.PathValue("id"), u.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			htmlutil.NotFoundPage(w, r, h.logger)
			return
		}
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	sess.Put("flash", MsgProjectDeleted)

	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (h *Handler) find(w http.ResponseWriter, r *http.Request, asJSON bool) (*Project, bool) {
	p, err := h.store.GetByPublicID(r.Context(), r.PathValue("id"))
	if err != nil && !errors.Is(err, ErrNotFound) {
		if asJSON {
			jsonutil.ServerErrorResponse(w, r, err, h.logger)
		} else {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
		}
		return nil, false
	}

	if p != nil && mayRead(r, p) {
		return p, true
	}

	if p != nil && p.IsProtected() && !asJSON {
		h.unlockPage(w, r, p, "")
		return nil, false
	}

	if asJSON {
		jsonutil.NotFoundResponse(w)
	} else {
		htmlutil.NotFoundPage(w, r, h.logger)
	}

	return nil, false
}

func (h *Handler) ConvertPage(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)

	var saved []*Project

	if !u.IsAnonymous() {
		var err error
		if saved, err = h.store.ListByUserID(r.Context(), u.ID, h.listLimit); err != nil {
			htmlutil.ServerErrorResponse(w, r, err, h.logger)
			return
		}
	}

	page := user.NewPage(r, saved)
	page.Examples = fixtures.Examples()
	page.Description = landing.BySlug("/mpp-to-excel").Description
	page.Canonical = h.baseURL + "/mpp-to-excel"
	page.Public = true

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Convert, page, h.logger)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	u := user.GetUserContext(r)
	if u.IsAnonymous() {
		http.Redirect(w, r, "/signin", http.StatusSeeOther)
		return
	}

	projects, err := h.store.ListByUserID(r.Context(), u.ID, h.listLimit)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	saved, err := h.store.CountByUserID(r.Context(), u.ID)
	if err != nil {
		htmlutil.ServerErrorResponse(w, r, err, h.logger)
		return
	}

	page := user.NewPage(r, projects)
	page.SavedNote = savedNote(u, saved, len(projects))

	htmlutil.WriteHTML(w, r, http.StatusOK, h.templates.Projects, page, h.logger)
}
