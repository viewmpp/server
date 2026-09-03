package user

import (
	"fmt"
	"time"
)

const (
	MsgEmailRequired    = "Enter your email address"
	MsgEmailTooLong     = "That address is too long"
	MsgEmailInvalid     = "That doesn't look like an email address"
	MsgEmailTaken       = "This address is already registered"
	MsgEmailNotFound    = "Could not find this account"
	MsgEmailNotVerified = "This email address hasn't been verified yet"
	MsgEmailOrPass      = "Incorrect email or password"

	MsgSubscribeNeedsEmail  = "Confirm your email address first"
	MsgProUnavailable       = "Sorry, Pro isn't available right now. Try again shortly"
	MsgPassRequired         = "Choose a password"
	MsgPassTooLong          = "That password is too long"
	MsgCodeRequired         = "Enter the code we emailed you"
	MsgCodeInvalid          = "That code is wrong or has expired"
	MsgTooManyTries         = "Too many attempts. Wait a minute and try again"
	MsgVerifyRetry          = "Sorry, something went wrong. Try again"
	MsgEmailConfirmed       = "Your email address has been confirmed."
	MsgPasswordChanged      = "Your password has been changed."
	MsgWrongCurrentPassword = "Your current password isn't right"
	MsgCodeUpdated          = "Your verification code has been updated and sent to you."
	MsgVerifyLater          = "Confirm your email address. Without it you can't get back in if you forget your password"
)

func MsgPassTooShort() string {
	return fmt.Sprintf("Use at least %d characters", MinPasswordLength)
}

func MsgEarlyAccessGranted(seat int, until time.Time) string {
	return fmt.Sprintf("You are one of our first %d users, so Pro is on us. "+
		"It is yours until %s, and you can renew it from your account when it runs out.",
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
