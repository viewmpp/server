package htmlutil

import (
	"fmt"
	"html/template"
	"server/ui"
)

type Templates struct {
	App *template.Template
}

func NewTemplates() (*Templates, error) {
	app, err := parsePage("app.tmpl")
	if err != nil {
		return nil, err
	}

	return &Templates{App: app}, nil
}

func parsePage(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, "templates/base.tmpl", fmt.Sprintf("templates/pages/%s", name))
}
