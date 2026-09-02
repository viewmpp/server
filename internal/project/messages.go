package project

import (
	"fmt"
	"server/internal/user"
)

const (
	MsgNowPublic       = "Anyone with the link can now open this plan."
	MsgNowPrivate      = "The link no longer works. Only you can open this plan."
	MsgProjectDeleted  = "Project deleted."
	MsgNowProtected    = "The link now asks for the password before opening the plan."
	MsgWrongPassword   = "Wrong password"
	MsgTooManyTries    = "Too many attempts, wait a minute"
	MsgConfirmEmail    = "Confirm your email address to share a link."
	MsgProtectNeedsPro = "Password-protected links are a Pro feature. Public links stay free."
)

func MsgPasswordLength(min int) string {
	return fmt.Sprintf("The password must be at least %d characters.", min)
}

func MsgShareLimit(limit int) string {
	return fmt.Sprintf("Free accounts can share %d projects at a time. Stop sharing one, or go Pro for unlimited links.", limit)
}

func MsgSaveLimit(saved int) string {
	return fmt.Sprintf("You have %d saved plans, the most a free account keeps. Delete one, or go Pro to save without a limit.", saved)
}

func savedNote(u *user.User, saved, shown int) string {
	if saved > shown {
		return fmt.Sprintf("Showing the %d most recent of %d saved plans.", shown, saved)
	}

	if u.HasSubscription() {
		return ""
	}

	return fmt.Sprintf("%d of %d saved plans used.", saved, user.MaxSavedFree)
}

func protectRefusal(u *user.User) string {
	if !u.Verified {
		return MsgConfirmEmail
	}
	return MsgProtectNeedsPro
}
