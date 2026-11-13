package mailer

import "time"

type CodeEmailData struct {
	Subject string
	Message string
	Hint    string
	Code    string
	Site    string
}

type ExistingAccountData struct {
	Subject string
	Message string
	Hint    string
	Site    string
}

func (m *Mailer) SendVerification(email, code string) error {
	data := CodeEmailData{
		Subject: "Confirm your email address - View MPP",
		Message: "To finish creating your View MPP account, use the confirmation code below.",
		Hint:    "If you did not sign up, ignore this email and nothing will happen.",
		Code:    code,
		Site:    m.site,
	}

	html, err := m.renderTemplate(m.templates.Verification, data)
	if err != nil {
		return err
	}

	text := plainText(data.Message, data.Code, data.Hint, m.signature())

	return m.send(data.Subject, html, text, email, "code", code)
}

type ResetEmailData struct {
	Subject string
	Message string
	Hint    string
	Link    string
	Site    string
}

func (m *Mailer) SendPasswordReset(email, link string) error {
	data := ResetEmailData{
		Subject: "Reset your View MPP password",
		Message: "Use the link below to choose a new password for your View MPP account. It expires shortly.",
		Hint:    "If you did not ask to reset your password, ignore this email - nothing has changed.",
		Link:    link,
		Site:    m.site,
	}

	html, err := m.renderTemplate(m.templates.PasswordReset, data)
	if err != nil {
		return err
	}

	text := plainText(data.Message, data.Link, data.Hint, m.signature())

	return m.send(data.Subject, html, text, email, "link", link)
}

func (m *Mailer) SendExistingAccount(email string) error {
	data := ExistingAccountData{
		Subject: "A sign-up attempt on View MPP",
		Message: "Someone tried to create a View MPP account with this email address. It already has one, so nothing was created.",
		Hint:    "If this wasn't you, you can safely ignore this email. Your account is untouched.",
		Site:    m.site,
	}

	html, err := m.renderTemplate(m.templates.AccountExists, data)
	if err != nil {
		return err
	}

	text := plainText(data.Message, data.Hint, m.signature())

	return m.send(data.Subject, html, text, email)
}

type TrialEndingData struct {
	Subject string
	Message string
	Detail  string
	Hint    string
	Link    string
	Site    string
}

func (m *Mailer) SendTrialEnding(email string, endsAt time.Time) error {
	data := TrialEndingData{
		Subject: "Your View MPP Pro ends " + endsAt.UTC().Format("2 January"),
		Message: "Your free Pro on View MPP ends on " + endsAt.UTC().Format("2 January 2006, 15:04") + " UTC. " +
			"To keep it, choose a plan: $6.99 a month or $59 a year, cancel any time.",
		Detail: "If you do nothing, the account moves to the free plan. No plan is deleted, but password-protected links " +
			"turn private, sharing goes back to 2 links at a time and saving stops above 20 plans.",
		Hint: "This is the only reminder we send about it.",
		Link: m.site + "/pricing",
		Site: m.site,
	}

	html, err := m.renderTemplate(m.templates.TrialEnding, data)
	if err != nil {
		return err
	}

	text := plainText(data.Message, data.Detail, data.Link, data.Hint, m.signature())

	return m.send(data.Subject, html, text, email)
}
