package project

import (
	"fmt"
	"server/internal/user"
)

const (
	MsgNowPublic       = "Anyone with the link can now open this plan."
	MsgNowPrivate      = "The link no longer works. Only you can open this plan."
	MsgProjectDeleted  = "The plan has been deleted."
	MsgNowProtected    = "The link now asks for a password before it opens the plan."
	MsgWrongPassword   = "Wrong password"
	MsgTooManyTries    = "Too many attempts. Wait a minute and try again"
	MsgConfirmEmail    = "Confirm your email address to share a link."
	MsgProtectNeedsPro = "Password-protected links are a Pro feature. Public links stay free."
)

func MsgPasswordLength(min int) string {
	return fmt.Sprintf("Use at least %d characters.", min)
}

func MsgShareLimit(limit int) string {
	return fmt.Sprintf("Free accounts share %d plans at a time. Stop sharing one, or go Pro for as many links as you like.", limit)
}

func MsgSaveLimit(saved int) string {
	return fmt.Sprintf("You have %d saved plans, which is all a free account holds. Delete one, or go Pro to save as many as you like.", saved)
}

func savedNote(u *user.User, saved, shown int) string {
	if saved > shown {
		return fmt.Sprintf("Showing your %d most recent plans of %d saved.", shown, saved)
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
