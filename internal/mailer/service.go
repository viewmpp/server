package mailer

type CodeEmailData struct {
	Subject string
	Message string
	Hint    string
	Code    string
}

type ExistingAccountData struct {
	Subject string
	Message string
	Hint    string
}

func (m *Mailer) SendVerification(email, code string) error {
	data := CodeEmailData{
		Subject: "Email Verification",
		Message: "To complete your registration, please use the confirmation code:",
		Hint:    "If you did not request registration, ignore this email.",
		Code:    code,
	}

	html, err := m.renderTemplate(m.templates.Verification, data)
	if err != nil {
		return err
	}

	return m.send(data.Subject, html, email)
}

type ResetEmailData struct {
	Subject string
	Message string
	Hint    string
	Link    string
}

func (m *Mailer) SendPasswordReset(email, link string) error {
	data := ResetEmailData{
		Subject: "Reset your password",
		Message: "Use the link below to choose a new password. It expires shortly.",
		Hint:    "If you did not ask to reset your password, ignore this email - nothing has changed.",
		Link:    link,
	}

	html, err := m.renderTemplate(m.templates.PasswordReset, data)
	if err != nil {
		return err
	}

	return m.send(data.Subject, html, email)
}

func (m *Mailer) SendExistingAccount(email string) error {
	data := ExistingAccountData{
		Subject: "Registration attempt",
		Message: "Someone tried to register an account using this email address.",
		Hint:    "If this wasn't you, you can safely ignore this email.",
	}

	html, err := m.renderTemplate(m.templates.AccountExists, data)
	if err != nil {
		return err
	}

	return m.send(data.Subject, html, email)
}
