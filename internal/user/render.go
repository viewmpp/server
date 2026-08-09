package user

import (
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"server/internal/htmlutil"
	"server/internal/session"
	"server/internal/vcs"
)

func render(w http.ResponseWriter, r *http.Request, status int, tmpl *template.Template, form any, logger *slog.Logger) {
	sess := session.FromContext(r)

	u := GetUserContext(r)

	page := htmlutil.Page{
		Version:   url.QueryEscape(vcs.Version()),
		CSRFToken: sess.CSRF(),
		Flash:     sess.Pop("flash"),
		Form:      form,
	}

	if !u.IsAnonymous() {
		page.UserEmail = u.Email
		page.Verified = u.Verified
	}

	if err := htmlutil.WriteHTML(w, status, tmpl, page); err != nil {
		htmlutil.ServerErrorResponse(w, r, err, logger)
	}
}
