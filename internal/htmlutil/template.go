package htmlutil

import (
	"fmt"
	"html/template"
	"server/ui"
)

type Templates struct {
	Upload *template.Template
	Viewer *template.Template
}

func NewTemplates() (*Templates, error) {
	upload, err := parsePage("upload.tmpl")
	if err != nil {
		return nil, err
	}

	viewer, err := parsePage("viewer.tmpl")
	if err != nil {
		return nil, err
	}

	return &Templates{
		Upload: upload,
		Viewer: viewer,
	}, nil
}

func parsePage(name string) (*template.Template, error) {
	return template.ParseFS(ui.Files, "html/base.tmpl", fmt.Sprintf("html/pages/%s", name))
}
