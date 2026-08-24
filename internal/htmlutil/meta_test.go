package htmlutil

import (
	"bytes"
	"html/template"
	"regexp"
	"server/internal/assert"
	"server/internal/landing"
	"testing"
)

const (
	maxTitle       = 60
	maxDescription = 155
)

var (
	titlePattern  = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	descPattern   = regexp.MustCompile(`<meta name="description" content="([^"]*)"`)
	robotsPattern = regexp.MustCompile(`<meta name="robots" content="([^"]*)"`)
)

func indexable(t *testing.T) map[string]*template.Template {
	t.Helper()

	pages, err := NewPages()
	assert.NilError(t, err)

	return map[string]*template.Template{
		"/":                                 pages.App,
		"/examples":                         pages.Examples,
		"/mpp-to-excel":                     pages.Convert,
		"/pricing":                          pages.Pricing,
		"/share-a-project-plan":             pages.Share,
		"/privacy":                          pages.Privacy,
		"/terms":                            pages.Terms,
		"/open-mpp-file-without-ms-project": pages.WithoutProject,
		"/mpp-viewer-mac":                   pages.Mac,
	}
}

func renderPage(t *testing.T, tmpl *template.Template, page Page) string {
	t.Helper()

	var buf bytes.Buffer
	assert.NilError(t, tmpl.ExecuteTemplate(&buf, "base", page))

	return buf.String()
}

func TestIndexablePagesFitTheSearchResult(t *testing.T) {
	for slug, tmpl := range indexable(t) {
		t.Run(slug, func(t *testing.T) {
			out := renderPage(t, tmpl, Page{Description: landing.BySlug(slug).Description, Public: true})

			title := titlePattern.FindStringSubmatch(out)
			if title == nil {
				t.Fatal("no title")
			}
			if n := len(title[1]); n > maxTitle {
				t.Errorf("title is %d characters, limit %d: the tail is cut off in the search result\n  %s", n, maxTitle, title[1])
			}

			desc := descPattern.FindStringSubmatch(out)
			if desc == nil {
				t.Fatal("no description")
			}
			if n := len(desc[1]); n > maxDescription {
				t.Errorf("description is %d characters, limit %d: the cut half usually holds the reason to click\n  %s", n, maxDescription, desc[1])
			}
		})
	}
}

func TestOnlyPublicPagesAreIndexable(t *testing.T) {
	pages, err := NewPages()
	assert.NilError(t, err)

	private := map[string]*template.Template{
		"signin": pages.Signin, "signup": pages.Signup, "verify": pages.Verify,
		"forgot": pages.Forgot, "reset": pages.Reset, "unlock": pages.Unlock,
		"projects": pages.Projects, "account": pages.Account,
	}

	for name, tmpl := range private {
		t.Run(name, func(t *testing.T) {
			page := Page{Form: stubForm{}}
			if name == "projects" {
				page.Form = projects()
			}

			out := renderPage(t, tmpl, page)

			robots := robotsPattern.FindStringSubmatch(out)
			if robots == nil || robots[1] != "noindex" {
				t.Fatalf("%s is missing noindex: a page behind sign-in must never be crawled", name)
			}
		})
	}

	for slug, tmpl := range indexable(t) {
		t.Run(slug, func(t *testing.T) {
			out := renderPage(t, tmpl, Page{Public: true})

			if robotsPattern.MatchString(out) {
				t.Fatalf("%s carries noindex while being a public landing page", slug)
			}
		})
	}
}
