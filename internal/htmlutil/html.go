package htmlutil

import (
	"bytes"
	"html/template"
	"net/http"
)

type Page struct {
	MaxUpload int64
	Version   string
}

func WriteHTML(w http.ResponseWriter, status int, ts *template.Template, page Page) error {
	buf := new(bytes.Buffer)
	if err := ts.ExecuteTemplate(buf, "base", page); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)

	return nil
}
