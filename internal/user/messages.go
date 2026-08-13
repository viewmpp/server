package user

import "fmt"

const (
	MsgEmailRequired = "Enter your email address"
	MsgEmailTooLong  = "That address is too long"
	MsgEmailInvalid  = "That does not look like an email address"
	MsgEmailTaken    = "This address is already registered"
	MsgEmailOrPass   = "Wrong email or password"

	MsgSubscribeNeedsEmail = "Confirm your email address first"
	MsgEarlyAccessClosed   = "All early access seats are taken"
	MsgPassRequired        = "Choose a password"
	MsgPassTooShort        = "Password must be at least 8 characters"
	MsgPassTooLong         = "Password is too long"
	MsgCodeRequired        = "Enter the code from the email"
	MsgCodeInvalid         = "That code is wrong or has expired"
	MsgTooManyTries        = "Too many attempts, wait a minute"
	MsgSignupRetry         = "Could not finish signing up, please try again"
	MsgVerifyRetry         = "Could not finish, please try again"
	MsgEmailConfirmed      = "Email confirmed"
	MsgVerifyLater         = "Confirm your email — without it you cannot get back in if you forget your password"
)

func MsgEarlyAccessGranted(seat int) string {
	return fmt.Sprintf("Pro is on — you are early user #%d. It stays free for you.", seat)
}

func MsgCodeSent(email string) string {
	return "We sent a code to " + email
}

func MsgCodeResent(email string) string {
	return "If the code is still valid, we sent it again to " + email
}
