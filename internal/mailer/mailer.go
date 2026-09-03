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
	prod      bool
}

func New(
	config config.Resend,
	templates *htmlutil.Templates,
	logger *slog.Logger,
	prod bool,
) *Mailer {

	if !prod {
		logger.Warn("mail goes to the log, not to anyone")
	}

	return &Mailer{
		client:    resend.NewClient(config.ResendAPIKey),
		sender:    config.ResendSender,
		templates: templates,
		logger:    logger,
		prod:      prod,
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

func (m *Mailer) send(subject, html, recipient string, detail ...any) error {

	if !m.prod {
		m.logger.With("to", recipient, "subject", subject).Info("dev mode, email not sent, log transport", detail...)
		return nil
	}

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
