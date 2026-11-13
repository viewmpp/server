package htmlutil

import (
	"bytes"
	"html/template"
	"server/internal/assert"
	"server/internal/examples"
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
		{"updated link", Page{ProjectID: "abc123", Access: "public", PreviousFileName: "plan rev13.mpp", UpdatedOn: "12 Nov 2026", UpdatedAt: "2026-11-12T21:20:59Z"}},
		{"example page", Page{ExampleName: "viaduct", ExampleLabel: "Viaduct", FileName: "виадук.mpp"}},
		{"with flash", Page{UserEmail: "a@b.c", Flash: "done", SavedNote: "20 of 20"}},
		{"billing, anonymous", Page{Billing: true, TrialDays: 7}},
		{"billing, trial open", Page{UserEmail: "a@b.c", Verified: true, Billing: true, TrialOpen: true, TrialDays: 7}},
		{"billing, on trial", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, TrialDays: 7, ProUntil: "20 Nov 2026", ProWarning: "ends in 7 days"}},
		{"billing, paid", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, PaidStatus: "active", PaidDate: "13 Dec 2026", CheckoutDone: true}},
		{"billing, cancelled", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, PaidStatus: "cancelled", PaidYearly: true, PaidDate: "13 Nov 2027"}},
		{"billing, past due", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, PaidStatus: "past_due"}},
	}

	templates := map[string]*template.Template{
		"app": pages.App, "convert": pages.Convert, "examples": pages.Examples,
		"projects": pages.Projects, "account": pages.Account, "pricing": pages.Pricing,
		"share":   pages.Share,
		"cookies": pages.Cookies,
		"privacy": pages.Privacy, "terms": pages.Terms, "signin": pages.Signin,
		"signup": pages.Signup, "verify": pages.Verify, "forgot": pages.Forgot,
		"reset": pages.Reset, "unlock": pages.Unlock, "mac": pages.Mac,
		"without-project": pages.WithoutProject,
	}

	for name, tmpl := range templates {
		for _, state := range states {
			t.Run(name+"/"+state.name, func(t *testing.T) {
				page := state.page
				page.Examples = examples.All()

				switch name {
				case "app", "examples":
					page.Form = examples.All()
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

func renderState(t *testing.T, tmpl *template.Template, page Page) string {
	t.Helper()
	page.sess = &session.Session{}
	if page.Form == nil {
		page.Form = stubForm{}
	}
	var buf bytes.Buffer
	assert.NilError(t, tmpl.ExecuteTemplate(&buf, "base", page))
	return buf.String()
}

func TestEachAccountStateOffersTheRightAction(t *testing.T) {
	pages, err := NewPages()
	assert.NilError(t, err)

	const (
		trial    = `action="/subscribe"`
		checkout = `action="/billing/checkout"`
		portal   = `href="/billing/portal"`
	)

	tests := []struct {
		name    string
		page    Page
		want    []string
		without []string
	}{
		{"before launch, free", Page{UserEmail: "a@b.c", Verified: true},
			[]string{trial, "Upgrade to Pro"}, []string{checkout, portal}},
		{"trial still open", Page{UserEmail: "a@b.c", Verified: true, Billing: true, TrialOpen: true, TrialDays: 7},
			[]string{trial, "Start a 7-day free trial", checkout, `value="monthly"`, `value="yearly"`}, []string{portal}},
		{"trial used up", Page{UserEmail: "a@b.c", Verified: true, Billing: true, TrialDays: 7},
			[]string{checkout}, []string{trial, portal}},
		{"on trial", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, ProUntil: "20 Nov 2026"},
			[]string{checkout, "free until 20 Nov 2026"}, []string{trial, portal}},
		{"paying", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, PaidStatus: "active", PaidDate: "13 Dec 2026"},
			[]string{portal, "monthly, renews 13 Dec 2026"}, []string{trial, checkout}},
		{"cancelled", Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true, PaidStatus: "cancelled", PaidDate: "13 Dec 2026"},
			[]string{portal, "Pro stays until 13 Dec 2026"}, []string{checkout}},
		{"unconfirmed email", Page{UserEmail: "a@b.c", Billing: true, TrialDays: 7},
			[]string{`href="/verify"`}, []string{checkout}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderState(t, pages.Account, tt.page)
			for _, s := range tt.want {
				if !strings.Contains(body, s) {
					t.Errorf("missing %s", s)
				}
			}
			for _, s := range tt.without {
				if strings.Contains(body, s) {
					t.Errorf("offers %s, which this state must not see", s)
				}
			}
		})
	}
}

func TestThePricingPageSellsOnlyOncePaymentsAreOffered(t *testing.T) {
	pages, err := NewPages()
	assert.NilError(t, err)

	before := renderState(t, pages.Pricing, Page{UserEmail: "a@b.c", Verified: true})
	if strings.Contains(before, `action="/billing/checkout"`) || strings.Contains(before, "free trial") {
		t.Error("the pricing page sells Pro before payments are offered")
	}
	if strings.Contains(before, "plans__was") {
		t.Error("a struck-through price that was never charged is back on the page")
	}

	anonymous := renderState(t, pages.Pricing, Page{Billing: true, TrialDays: 7})
	if !strings.Contains(anonymous, `href="/signup">Start a 7-day free trial`) {
		t.Error("a visitor is not invited to start the trial")
	}

	pro := renderState(t, pages.Pricing, Page{UserEmail: "a@b.c", Verified: true, Pro: true, Billing: true})
	if strings.Contains(pro, `action="/billing/checkout"`) {
		t.Error("a Pro user is offered a second purchase on the pricing page")
	}
}
