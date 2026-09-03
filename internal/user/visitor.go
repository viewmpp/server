package user

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"

	"server/internal/clientip"
	"server/internal/session"
)

func VisitorKey(r *http.Request, prefix string) string {
	if u := GetUserContext(r); !u.IsAnonymous() {
		return prefix + "user:" + strconv.FormatInt(u.ID, 10)
	}

	if sess := session.FromContext(r); sess.Established() {
		sum := sha256.Sum256([]byte(sess.Token))
		return prefix + "sess:" + base64.RawURLEncoding.EncodeToString(sum[:12])
	}

	return prefix + "ip:" + clientip.From(r)
}
