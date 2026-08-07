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

	html, err := renderTemplate("email_verification.tmpl", data)
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

	html, err := renderTemplate("existing_account.tmpl", data)
	if err != nil {
		return err
	}

	return m.send(data.Subject, html, email)
}
