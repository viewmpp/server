package htmlutil

import (
	"fmt"
	"html/template"
	"server/ui"
)

type Templates struct {
	App      *template.Template
	Signup   *template.Template
	Verify   *template.Template
	Signin   *template.Template
	Projects *template.Template
	Email
}

type Email struct {
	Verification  *template.Template
	AccountExists *template.Template
}

func NewTemplates() (*Templates, error) {
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

	verification, err := parseMail("email_verification.tmpl")
	if err != nil {
		return nil, err
	}

	accExists, err := parseMail("email_account_exists.tmpl")
	if err != nil {
		return nil, err
	}

	return &Templates{
		App:      app,
		Signup:   signup,
		Verify:   verify,
		Signin:   signin,
		Projects: projects,
		Email: Email{
			Verification:  verification,
			AccountExists: accExists,
		},
	}, nil
}

func parsePage(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, "templates/base.tmpl", fmt.Sprintf("templates/pages/%s", name))
}

func parseMail(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, fmt.Sprintf("templates/emails/%s", name))
}
