package user

import (
	"fmt"
	"time"
)

const (
	MsgEmailRequired    = "Enter your email address"
	MsgEmailTooLong     = "That address is too long"
	MsgEmailInvalid     = "That does not look like an email address"
	MsgEmailTaken       = "This address is already registered"
	MsgEmailNotFound    = "This address is not found"
	MsgEmailNotVerified = "This address is not verified"
	MsgEmailOrPass      = "Wrong email or password"

	MsgSubscribeNeedsEmail  = "Confirm your email address first"
	MsgProUnavailable       = "Pro is not available right now - please try again later"
	MsgPassRequired         = "Choose a password"
	MsgPassTooShort         = "Password must be at least 8 characters"
	MsgPassTooLong          = "Password is too long"
	MsgCodeRequired         = "Enter the code from the email"
	MsgCodeInvalid          = "That code is wrong or has expired"
	MsgTooManyTries         = "Too many attempts, wait a minute"
	MsgVerifyRetry          = "Could not finish, please try again"
	MsgEmailConfirmed       = "Email confirmed"
	MsgPasswordChanged      = "Password changed"
	MsgWrongCurrentPassword = "That is not your current password"
	MsgVerifyLater          = "Confirm your email - without it you cannot get back in if you forget your password"
)

func MsgEarlyAccessGranted(seat int, until time.Time) string {
	return fmt.Sprintf("Congratulations - you are one of our first %d users, so Pro is on us. "+
		"It runs until %s with nothing to pay, and you can renew it from your account when it ends.",
		seat, until.Format("2 January 2006"))
}

func MsgLinkLife(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}

	if d < 2*time.Hour {
		return "one hour"
	}

	return fmt.Sprintf("%d hours", int(d.Hours()))
}

func MsgCodeSent(email string) string {
	return "We sent a code to " + email
}

func MsgCodeResent(email string) string {
	return "If the code is still valid, we sent it again to " + email
}
