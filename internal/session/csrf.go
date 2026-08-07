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
	return s.Data[csrfKey]
}

func (s *Session) ensureCSRF() error {
	if s.Data[csrfKey] != "" {
		return nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}

	s.Data[csrfKey] = base64.RawURLEncoding.EncodeToString(raw)

	return nil
}

func VerifyCSRF(sess *Session, r *http.Request) bool {
	want := sess.CSRF()
	got := r.PostFormValue(CSRFField)

	if want == "" || got == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
