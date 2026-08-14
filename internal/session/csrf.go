package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const (
	csrfKey   = "csrf"
	CSRFField = "csrf_token"
)

func (s *Session) CSRF() string {
	if s.Data[csrfKey] == "" {
		raw := make([]byte, 32)
		_, _ = rand.Read(raw)

		s.Data[csrfKey] = base64.RawURLEncoding.EncodeToString(raw)
		s.touch()
	}

	return s.Data[csrfKey]
}

func VerifyCSRF(sess *Session, r *http.Request) bool {
	want := sess.Data[csrfKey]
	got := r.PostFormValue(CSRFField)

	if want == "" || got == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
