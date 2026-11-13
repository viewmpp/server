package mailer

import (
	"strings"
	"testing"
	"time"
)

func TestTheTrialReminderSaysWhenAndWhereToGo(t *testing.T) {
	m := testMailer(t)
	ends := time.Date(2026, 11, 20, 14, 0, 0, 0, time.UTC)

	html, err := m.renderTemplate(m.templates.TrialEnding, TrialEndingData{
		Subject: "Your View MPP Pro ends 20 November",
		Message: "Your free Pro on View MPP ends on " + ends.Format("2 January 2006, 15:04") + " UTC.",
		Detail:  "detail",
		Hint:    "hint",
		Link:    m.site + "/pricing",
		Site:    m.site,
	})
	if err != nil {
		t.Fatalf("the reminder did not render: %v", err)
	}

	for _, want := range []string{`href="https://viewmpp.com/pricing"`, "20 November 2026, 14:00 UTC", "Keep Pro", "detail"} {
		if !strings.Contains(html, want) {
			t.Errorf("the reminder is missing %q", want)
		}
	}
}

func TestTheTrialReminderGoesOutInDevAsALogLine(t *testing.T) {
	if err := testMailer(t).SendTrialEnding("a@b.c", time.Now().Add(20*time.Hour)); err != nil {
		t.Errorf("send = %v", err)
	}
}
