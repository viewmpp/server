package session

import (
	"errors"
	"maps"
	"net/http"
)

func (s *Store) Middleware(next http.Handler, onError func(http.ResponseWriter, *http.Request, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		sess, err := s.Find(r.Context(), w, r)
		if errors.Is(err, ErrNotFound) {
			sess, err = s.New(w)
		}
		if err != nil {
			onError(w, r, err)
			return
		}

		before := maps.Clone(sess.Data)
		beforeUser := userID(sess)

		next.ServeHTTP(w, SetContext(r, sess))

		if sess.dropped || (maps.Equal(before, sess.Data) && beforeUser == userID(sess)) {
			return
		}

		if err = s.Save(r.Context(), sess); err != nil {
			s.logger.Error("session not saved", "err", err, "path", r.URL.Path)
		}
	})
}

func userID(sess *Session) int64 {
	if sess.UserID == nil {
		return 0
	}
	return *sess.UserID
}
