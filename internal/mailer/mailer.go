package mailer

import (
	"bytes"
	"html/template"
	"log/slog"
	"server/internal/config"
	"server/internal/htmlutil"

	"github.com/resend/resend-go/v3"
)

type Mailer struct {
	client    *resend.Client
	sender    string
	templates *htmlutil.Templates
	logger    *slog.Logger
}

func New(
	config config.Resend,
	templates *htmlutil.Templates,
	logger *slog.Logger,
) *Mailer {
	return &Mailer{
		client:    resend.NewClient(config.APIKey),
		sender:    config.Sender,
		templates: templates,
		logger:    logger,
	}
}

func (m *Mailer) renderTemplate(tmpl *template.Template, data any) (string, error) {

	var html bytes.Buffer

	err := tmpl.ExecuteTemplate(&html, "html", data)
	if err != nil {
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
