package htmlutil

import (
	"fmt"
	"html/template"
	"server/internal/landing"
	"server/ui"
)

type Templates struct {
	*Pages
	*Emails
}

type Pages struct {
	App            *template.Template
	Signin         *template.Template
	Signup         *template.Template
	Verify         *template.Template
	Projects       *template.Template
	Unlock         *template.Template
	Examples       *template.Template
	Convert        *template.Template
	Forgot         *template.Template
	Reset          *template.Template
	Privacy        *template.Template
	Terms          *template.Template
	WithoutProject *template.Template
	Mac            *template.Template
}

type Emails struct {
	Verification  *template.Template
	AccountExists *template.Template
	PasswordReset *template.Template
}

type Errors struct {
	ServerError *template.Template
	BadRequest  *template.Template
	NotFound    *template.Template
}

func NewTemplates() (*Templates, error) {
	pages, err := NewPages()
	if err != nil {
		return nil, err
	}

	emails, err := NewEmails()
	if err != nil {
		return nil, err
	}

	return &Templates{
		Pages:  pages,
		Emails: emails,
	}, nil
}

func NewPages() (*Pages, error) {
	app, err := parsePage("app.tmpl")
	if err != nil {
		return nil, err
	}

	signup, err := parsePage("signup.tmpl")
	if err != nil {
		return nil, err
	}

	signin, err := parsePage("signin.tmpl")
	if err != nil {
		return nil, err
	}

	verify, err := parsePage("verify.tmpl")
	if err != nil {
		return nil, err
	}

	projects, err := parsePage("projects.tmpl")
	if err != nil {
		return nil, err
	}

	unlock, err := parsePage("unlock.tmpl")
	if err != nil {
		return nil, err
	}

	examples, err := parsePage("examples.tmpl")
	if err != nil {
		return nil, err
	}

	convert, err := parsePage("convert.tmpl")
	if err != nil {
		return nil, err
	}

	forgot, err := parsePage("forgot.tmpl")
	if err != nil {
		return nil, err
	}

	reset, err := parsePage("reset.tmpl")
	if err != nil {
		return nil, err
	}

	privacy, err := parsePage("privacy.tmpl")
	if err != nil {
		return nil, err
	}

	terms, err := parsePage("terms.tmpl")
	if err != nil {
		return nil, err
	}

	withoutProject, err := parsePage("without-project.tmpl")
	if err != nil {
		return nil, err
	}

	mac, err := parsePage("mac.tmpl")
	if err != nil {
		return nil, err
	}

	return &Pages{
		App:            app,
		Signin:         signin,
		Signup:         signup,
		Verify:         verify,
		Projects:       projects,
		Unlock:         unlock,
		Examples:       examples,
		Convert:        convert,
		Forgot:         forgot,
		Reset:          reset,
		Privacy:        privacy,
		Terms:          terms,
		WithoutProject: withoutProject,
		Mac:            mac,
	}, nil
}

func NewEmails() (*Emails, error) {
	verification, err := parseEmail("email_verification.tmpl")
	if err != nil {
		return nil, err
	}

	accExists, err := parseEmail("email_account_exists.tmpl")
	if err != nil {
		return nil, err
	}

	passwordReset, err := parseEmail("email_password_reset.tmpl")
	if err != nil {
		return nil, err
	}

	return &Emails{
		Verification:  verification,
		AccountExists: accExists,
		PasswordReset: passwordReset,
	}, nil
}

var funcs = template.FuncMap{"footerLinks": landing.Footer}

func parsePage(name string) (*template.Template, error) {
	return template.New(name).Funcs(funcs).ParseFS(ui.Files,
		"templates/base.tmpl",
		"templates/partials/viewer.tmpl",
		"templates/partials/footer.tmpl",
		fmt.Sprintf("templates/pages/%s", name))
}

func parseEmail(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, fmt.Sprintf("templates/emails/%s", name))
}
