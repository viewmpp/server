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
	Login    *template.Template
	Projects *template.Template
	Verf     *template.Template
	Exists   *template.Template
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

	verify, err := parsePage("verify.tmpl")
	if err != nil {
		return nil, err
	}

	login, err := parsePage("login.tmpl")
	if err != nil {
		return nil, err
	}

	projects, err := parsePage("projects.tmpl")
	if err != nil {
		return nil, err
	}

	verf, err := parseMail("email_verification.tmpl")
	if err != nil {
		return nil, err
	}

	exists, err := parseMail("existing_account.tmpl")
	if err != nil {
		return nil, err
	}

	return &Templates{
		App:      app,
		Signup:   signup,
		Verify:   verify,
		Login:    login,
		Projects: projects,
		Verf:     verf,
		Exists:   exists,
	}, nil
}

func parsePage(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, "templates/base.tmpl", fmt.Sprintf("templates/pages/%s", name))
}

func parseMail(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, fmt.Sprintf("templates/emails/%s", name))
}
