package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/fixtures"
	"server/internal/session"
	"server/internal/vcs"
	"strconv"
)

type Page struct {
	Title             string
	Description       string
	Canonical         string
	NoIndex           bool
	Public            bool
	MaxUpload         int64
	Version           string
	Flash             string
	Form              any
	Examples          []fixtures.Example
	SavedNote         string
	UserEmail         string
	Verified          bool
	Pro               bool
	ProjectID         string
	ExampleName       string
	ExampleLabel      string
	FileName          string
	Access            string
	IsOwner           bool
	CanShare          bool
	MaxPublicFree     int
	MinPasswordLength int
	MaxPasswordLength int

	sess *session.Session
}

func (p Page) CSRFToken() string {
	if p.sess == nil {
		return ""
	}
	return p.sess.CSRF()
}

func AcceptPost(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (*session.Session, bool) {
	sess := session.FromContext(r)

	if err := r.ParseForm(); err != nil {
		BadRequestPage(w, r, logger)
		return nil, false
	}

	if !session.VerifyCSRF(sess, r) {
		BadRequestPage(w, r, logger)
		return nil, false
	}

	return sess, true
}

func WriteHTML(w http.ResponseWriter, r *http.Request, status int, ts *template.Template, page Page, logger *slog.Logger) {
	sess := session.FromContext(r)

	page.Version = url.QueryEscape(vcs.Version())
	page.Flash = sess.Pop("flash")
	page.sess = sess

	buf := new(bytes.Buffer)
	if err := ts.ExecuteTemplate(buf, "base", page); err != nil {
		ServerErrorResponse(w, r, err, logger)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl(page, sess))
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (p Page) MaxUploadLabel() string {
	return strconv.FormatInt(p.MaxUpload>>20, 10) + " MB"
}

func cacheControl(page Page, sess *session.Session) string {
	if page.Public && sess.UserID == nil {
		return "public, max-age=300"
	}
	return "no-store"
}
