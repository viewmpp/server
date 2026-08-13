package htmlutil

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/session"
	"server/internal/vcs"
)

type Page struct {
	MaxUpload    int64
	Version      string
	Flash        string
	Form         any
	UserEmail    string
	Verified     bool
	Pro          bool
	ProjectID    string
	ExampleName  string
	ExampleLabel string
	FileName     string
	Access       string
	IsOwner      bool
	CanShare     bool

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
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
