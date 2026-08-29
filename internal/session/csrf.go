package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const CSRFField = "csrf_token"

func (s *Session) CSRF() string {
	s.csrfUsed = true

	if s.csrf != "" {
		return s.csrf
	}

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)

	s.csrf = base64.RawURLEncoding.EncodeToString(raw)

	if s.store != nil && s.w != nil {
		s.store.setCSRFCookie(s.w, s.csrf, s.ExpiresAt)
	}

	return s.csrf
}

func (s *Session) CSRFUsed() bool {
	return s.csrfUsed
}

func VerifyCSRF(sess *Session, r *http.Request) bool {
	want := sess.csrf
	got := r.PostFormValue(CSRFField)

	if want == "" || got == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
