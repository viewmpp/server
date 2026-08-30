package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
)

const CSRFField = "csrf_token"

func (s *Session) CSRF() string {
	s.csrfUsed = true

	if s.csrf == "" {
		raw := make([]byte, 32)
		_, _ = rand.Read(raw)

		s.csrf = base64.RawURLEncoding.EncodeToString(raw)

		if s.store != nil && s.w != nil {
			s.store.setCSRFCookie(s.w, s.csrf, s.ExpiresAt)
		}
	}

	s.touch()

	return s.sign(s.csrf)
}

func (s *Session) CSRFUsed() bool {
	return s.csrfUsed
}

func (s *Session) sign(value string) string {
	if s.store == nil {
		return ""
	}

	mac := hmac.New(sha256.New, s.store.secret)
	mac.Write([]byte(value))
	mac.Write([]byte("|"))
	mac.Write([]byte(s.Token))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyCSRF(sess *Session, r *http.Request) bool {
	got := r.PostFormValue(CSRFField)

	if sess.csrf == "" || got == "" {
		return false
	}

	want := sess.sign(sess.csrf)
	if want == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
