package htmlutil

import (
	"bytes"
	"server/internal/session"
	"strings"
	"testing"
)

func TestCacheControl(t *testing.T) {
	signedIn := int64(7)

	cases := []struct {
		name string
		page Page
		sess *session.Session
		want string
	}{
		{"default", Page{}, &session.Session{}, "no-store"},
		{"public, anonymous", Page{Public: true}, &session.Session{}, publicMaxAge},
		{"public, signed in", Page{Public: true}, &session.Session{UserID: &signedIn}, "no-store"},
		{"private, signed in", Page{}, &session.Session{UserID: &signedIn}, "no-store"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cacheControl(c.page, c.sess); got != c.want {
				t.Errorf("cacheControl() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestZeroPageKeepsCurrentHead(t *testing.T) {
	head := render(t, Page{})

	for _, unwanted := range []string{`name="robots"`, `rel="canonical"`} {
		if strings.Contains(head, unwanted) {
			t.Errorf("zero Page emitted %s", unwanted)
		}
	}

	if !strings.Contains(head, "Open an MS Project (.mpp) file in your browser") {
		t.Error("default description is gone")
	}

	if !strings.Contains(head, "<title>MPP Viewer — open an MS Project plan without installing anything</title>") {
		t.Error("title from the page template is gone")
	}
}

func TestPageFieldsReachTheHead(t *testing.T) {
	head := render(t, Page{
		Title:       "Custom title",
		Description: "Custom description",
		Canonical:   "https://example.com/x",
		NoIndex:     true,
	})

	for _, want := range []string{
		"<title>Custom title</title>",
		`<meta name="description" content="Custom description">`,
		`<meta name="robots" content="noindex">`,
		`<link rel="canonical" href="https://example.com/x">`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("missing from head: %s", want)
		}
	}

	if strings.Contains(head, "Open an MS Project (.mpp) file in your browser") {
		t.Error("default description was not replaced")
	}
}

func render(t *testing.T, page Page) string {
	t.Helper()

	tmpl, err := parsePage("app.tmpl")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var buf bytes.Buffer
	if err = tmpl.ExecuteTemplate(&buf, "base", page); err != nil {
		t.Fatalf("execute: %v", err)
	}

	return buf.String()
}
