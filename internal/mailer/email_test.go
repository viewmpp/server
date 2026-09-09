package mailer

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"server/internal/htmlutil"
)

func testMailer(t *testing.T) *Mailer {
	t.Helper()

	emails, err := htmlutil.NewEmails()
	if err != nil {
		t.Fatalf("email templates did not parse: %v", err)
	}

	return &Mailer{
		sender:    "noreply@viewmpp.com",
		site:      "https://viewmpp.com",
		templates: &htmlutil.Templates{Emails: emails},
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestEveryEmailSaysWhoSentIt(t *testing.T) {
	m := testMailer(t)

	rendered := map[string]string{}

	for name, render := range map[string]func() (string, error){
		"verification": func() (string, error) {
			return m.renderTemplate(m.templates.Verification, CodeEmailData{
				Subject: "Confirm your email address - View MPP",
				Message: "message", Hint: "hint", Code: "123456", Site: m.site,
			})
		},
		"password reset": func() (string, error) {
			return m.renderTemplate(m.templates.PasswordReset, ResetEmailData{
				Subject: "Reset your View MPP password",
				Message: "message", Hint: "hint",
				Link: "https://viewmpp.com/reset/token", Site: m.site,
			})
		},
		"account exists": func() (string, error) {
			return m.renderTemplate(m.templates.AccountExists, ExistingAccountData{
				Subject: "A sign-up attempt on View MPP",
				Message: "message", Hint: "hint", Site: m.site,
			})
		},
	} {
		out, err := render()
		if err != nil {
			t.Fatalf("%s did not render: %v", name, err)
		}
		rendered[name] = out
	}

	for name, out := range rendered {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(out, "View MPP") {
				t.Error("the message never names the product: an unsigned email carrying a code is what phishing looks like")
			}

			if !strings.Contains(out, m.site) {
				t.Errorf("the message does not link back to %s, so the reader cannot tell where it came from", m.site)
			}
		})
	}

	if !strings.Contains(rendered["verification"], "123456") {
		t.Error("the confirmation code is missing from the verification email")
	}

	if !strings.Contains(rendered["password reset"], "https://viewmpp.com/reset/token") {
		t.Error("the reset link is missing from the password reset email")
	}
}

func TestTheSenderCarriesADisplayName(t *testing.T) {
	m := testMailer(t)

	if got := m.from(); got != "View MPP <noreply@viewmpp.com>" {
		t.Errorf("from = %q, want the address behind a readable name", got)
	}

	already := &Mailer{sender: "Someone <hi@viewmpp.com>"}
	if got := already.from(); got != "Someone <hi@viewmpp.com>" {
		t.Errorf("from = %q, want the configured value left alone when it already has a name", got)
	}
}

func TestThePlainTextPartCarriesTheEssentials(t *testing.T) {
	m := testMailer(t)

	got := plainText("Use the code below.", "123456", "", m.signature())

	for _, want := range []string{"Use the code below.", "123456", "View MPP", "https://viewmpp.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("the text part is missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "\n\n\n") {
		t.Errorf("an empty section left a gap in the text part:\n%q", got)
	}
}

func TestEveryMessageSurvivesTheWholePath(t *testing.T) {
	m := testMailer(t)

	for name, send := range map[string]func() error{
		"verification":   func() error { return m.SendVerification("someone@example.com", "123456") },
		"password reset": func() error { return m.SendPasswordReset("someone@example.com", "https://viewmpp.com/reset/tok") },
		"account exists": func() error { return m.SendExistingAccount("someone@example.com") },
	} {
		if err := send(); err != nil {
			t.Errorf("%s failed before it ever reached the transport: %v", name, err)
		}
	}
}
