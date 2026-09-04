package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/examples"
	"server/internal/session"
	"server/internal/vcs"
	"strconv"
	"strings"
)

var baseURL string

func SetBaseURL(u string) {
	baseURL = strings.TrimRight(u, "/")
}

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
	Examples          []examples.Example
	SavedNote         string
	SavedCount        int
	SharedCount       int
	ProUntil          string
	ProWarning        string
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
	CanProtect        bool
	MaxPublicFree     int
	MinPasswordLength int
	ResendSeconds     int
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

	page.Version = url.QueryEscape(vcs.Version)
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

func (p Page) OGImage() string {
	if baseURL == "" {
		return ""
	}

	return baseURL + "/static/icons/og.png?v=" + p.Version
}

func (p Page) Robots() string {
	if p.NoIndex || !p.Public {
		return "noindex"
	}

	return ""
}

func (p Page) MaxUploadLabel() string {
	return strconv.FormatInt(p.MaxUpload>>20, 10) + " MB"
}

func cacheControl(page Page, sess *session.Session) string {
	if page.Public && sess.UserID == nil && !sess.CSRFUsed() {
		return "public, max-age=300"
	}
	return "no-store"
}
