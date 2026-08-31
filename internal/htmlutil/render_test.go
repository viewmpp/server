package htmlutil

import (
	"bytes"
	"html/template"
	"server/internal/assert"
	"server/internal/fixtures"
	"server/internal/session"
	"strings"
	"testing"
	"time"
)

type stubProject struct {
	PublicID  string
	FileName  string
	Access    string
	CreatedAt time.Time
}

type stubForm struct {
	Token       string
	Email       string
	SentTo      string
	LinkLife    string
	EmailTaken  bool
	FileName    string
	FieldErrors map[string]string
}

func projects() []stubProject {
	return []stubProject{{PublicID: "abc123", FileName: "plan.mpp", Access: "public", CreatedAt: time.Now()}}
}

func TestEveryPageTemplateRenders(t *testing.T) {
	pages, err := NewPages()
	assert.NilError(t, err)

	form := stubForm{Token: "tok", Email: "a@b.c", EmailTaken: true, FileName: "plan.mpp", FieldErrors: map[string]string{"email": "bad", "password": "bad", "current": "bad", "confirm": "bad"}}

	states := []struct {
		name string
		page Page
	}{
		{"anonymous", Page{}},
		{"free unverified", Page{UserEmail: "a@b.c"}},
		{"free verified", Page{UserEmail: "a@b.c", Verified: true, CanShare: true}},
		{"pro", Page{UserEmail: "a@b.c", Verified: true, Pro: true, CanShare: true}},
		{"owner, private", Page{UserEmail: "a@b.c", Verified: true, ProjectID: "abc123", IsOwner: true, Access: "private", CanShare: true}},
		{"owner, shared", Page{UserEmail: "a@b.c", Verified: true, ProjectID: "abc123", IsOwner: true, Access: "public"}},
		{"example page", Page{ExampleName: "viaduct", ExampleLabel: "Viaduct", FileName: "виадук.mpp"}},
		{"with flash", Page{UserEmail: "a@b.c", Flash: "done", SavedNote: "20 of 20"}},
	}

	templates := map[string]*template.Template{
		"app": pages.App, "convert": pages.Convert, "examples": pages.Examples,
		"projects": pages.Projects, "account": pages.Account, "pricing": pages.Pricing,
		"share":   pages.Share,
		"privacy": pages.Privacy, "terms": pages.Terms, "signin": pages.Signin,
		"signup": pages.Signup, "verify": pages.Verify, "forgot": pages.Forgot,
		"reset": pages.Reset, "unlock": pages.Unlock, "mac": pages.Mac,
		"without-project": pages.WithoutProject,
	}

	for name, tmpl := range templates {
		for _, state := range states {
			t.Run(name+"/"+state.name, func(t *testing.T) {
				page := state.page
				page.Examples = fixtures.Examples()

				switch name {
				case "app", "examples":
					page.Form = fixtures.Examples()
				case "convert", "projects":
					page.Form = projects()
				default:
					page.Form = form
				}

				var buf bytes.Buffer
				if err := tmpl.ExecuteTemplate(&buf, "base", page); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if buf.Len() == 0 {
					t.Fatal("rendered nothing")
				}
			})
		}
	}
}

func TestTheResetPageDropsTheFormOnceTheLinkIsSent(t *testing.T) {
	pages, err := NewPages()
	assert.NilError(t, err)

	sent := Page{Form: stubForm{SentTo: "a@b.c", LinkLife: "one hour"}}

	var buf bytes.Buffer
	assert.NilError(t, pages.Forgot.ExecuteTemplate(&buf, "base", sent))

	body := buf.String()

	if !strings.Contains(body, "a@b.c") {
		t.Error("the address the link went to is not shown: it is the one thing worth checking on this screen")
	}

	if !strings.Contains(body, "one hour") {
		t.Error("the link lifetime is not shown")
	}

	if strings.Contains(body, `action="/reset"`) {
		t.Error("the email form is still on the page: it invites a second submission that the rate limit will refuse")
	}
}

func TestOnlyPagesWithoutAFormAreCachedPublicly(t *testing.T) {
	plain := &session.Session{}

	if got := cacheControl(Page{Public: true}, plain); got != "public, max-age=300" {
		t.Errorf("a public page without a form is %q, want it cacheable", got)
	}

	withForm := &session.Session{}
	withForm.CSRF()

	if got := cacheControl(Page{Public: true}, withForm); got != "no-store" {
		t.Errorf("cache-control = %q for a page carrying a csrf token: a shared cache would hand one token to every visitor", got)
	}
}
