package user

const (
	MsgEmailRequired  = "Enter your email address"
	MsgEmailTooLong   = "That address is too long"
	MsgEmailInvalid   = "That does not look like an email address"
	MsgEmailOrPass    = "Wrong email or password"
	MsgPassRequired   = "Choose a password"
	MsgPassTooShort   = "Password must be at least 8 characters"
	MsgPassTooLong    = "Password is too long"
	MsgCodeRequired   = "Enter the code from the email"
	MsgCodeInvalid    = "That code is wrong or has expired"
	MsgTooManyTries   = "Too many attempts, wait a minute"
	MsgSignupRetry    = "Could not finish signing up, please try again"
	MsgVerifyRetry    = "Could not finish, please try again"
	MsgEmailConfirmed = "Email confirmed"
	MsgVerifyLater    = "Confirm your email — without it you cannot get back in if you forget your password"
)

func MsgCodeSent(email string) string {
	return "We sent a code to " + email
}

func MsgCodeResent(email string) string {
	return "If the code is still valid, we sent it again to " + email
}
