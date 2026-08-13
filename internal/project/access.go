package project

import (
	"net/http"
	"server/internal/session"
	"server/internal/user"
)

func unlockKey(publicID string) string {
	return "unlocked:" + publicID
}

func unlocked(r *http.Request, publicID string) bool {
	return session.FromContext(r).Get(unlockKey(publicID)) != ""
}

func owns(u *user.User, p *Project) bool {
	return !u.IsAnonymous() && u.ID == p.UserID
}

func mayRead(r *http.Request, p *Project) bool {
	if owns(user.GetUserContext(r), p) {
		return true
	}
	if p.IsPublic() {
		return true
	}
	return p.IsProtected() && unlocked(r, p.PublicID)
}
