package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/resend/resend-go/v3"
)

//go:embed "templates"
var tmpl embed.FS

type Mailer struct {
	client *resend.Client
	sender string
	logger *slog.Logger
}

func New(apiKey string, sender string, logger *slog.Logger) *Mailer {
	return &Mailer{
		client: resend.NewClient(apiKey),
		sender: sender,
		logger: logger,
	}
}

func renderTemplate(tmplFile string, data any) (string, error) {
	ts, err := template.ParseFS(tmpl, fmt.Sprintf("templates/%s", tmplFile))
	if err != nil {
		return "", err
	}

	var html bytes.Buffer

	if err = ts.ExecuteTemplate(&html, "html", data); err != nil {
		return "", err
	}

	return html.String(), nil
}

func (m *Mailer) send(subject, html, recipient string) error {

	client := m.client

	params := &resend.SendEmailRequest{
		From:    m.sender,
		To:      []string{recipient},
		Subject: subject,
		Html:    html,
	}

	response, err := client.Emails.Send(params)
	if err != nil {
		m.logger.Error("failed to send email", "err", err)
		return err
	}
	m.logger.Info("email sent", "id", response.Id)
	return nil
}
