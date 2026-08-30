package project

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"server/internal/session"
	"server/internal/user"
)

func unlockKey(publicID string) string {
	return "unlocked:" + publicID
}

func unlockMark(passwordHash []byte) string {
	sum := sha256.Sum256(passwordHash)
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

func unlocked(r *http.Request, p *Project) bool {
	if len(p.Password) == 0 {
		return false
	}

	held := session.FromContext(r).Get(unlockKey(p.PublicID))

	return subtle.ConstantTimeCompare([]byte(held), []byte(unlockMark(p.Password))) == 1
}

func opensNewShare(current, next string) bool {
	return next != AccessPrivate && current == AccessPrivate
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
	return p.IsProtected() && unlocked(r, p)
}
