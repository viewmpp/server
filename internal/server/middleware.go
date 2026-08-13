package server

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/session"
	"server/internal/user"
	"strings"
	"time"
)

type CustomWriter struct {
	http.ResponseWriter
	statusCode int
}

func (cw *CustomWriter) WriteHeader(code int) {
	cw.statusCode = code
	cw.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		starts := time.Now()

		cw := &CustomWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(cw, r)

		status := cw.statusCode

		args := []any{
			slog.String("method", r.Method),
			slog.String("uri", r.RequestURI),
			slog.Int("status_code", status),
			slog.Duration("duration", time.Since(starts)),
		}

		switch {
		case status >= 500:
			s.logger.Error("request handled", args...)
		case status >= 400:
			s.logger.Warn("request handled", args...)
		default:
			s.logger.Info("request handled", args...)

		}
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				if strings.HasPrefix(r.URL.Path, "/api/") {
					jsonutil.ServerErrorResponse(w, r, fmt.Errorf("%v", err), s.logger)
					return
				}
				htmlutil.ServerErrorResponse(w, r, fmt.Errorf("%v", err), s.logger)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.FromContext(r)

		if sess.UserID == nil {
			next.ServeHTTP(w, user.SetUserContext(r, user.AnonymousUser))
			return
		}

		u, err := s.store.Users.GetByID(r.Context(), *sess.UserID)
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				sess.UserID = nil
				next.ServeHTTP(w, user.SetUserContext(r, user.AnonymousUser))
				return
			}
			htmlutil.ServerErrorResponse(w, r, err, s.logger)
			return
		}

		next.ServeHTTP(w, user.SetUserContext(r, u))
	})
}

func sessionUserID(sess *session.Session) int64 {
	if sess.UserID == nil {
		return 0
	}
	return *sess.UserID
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		sess, err := s.store.Sessions.Find(r.Context(), w, r)
		if errors.Is(err, session.ErrNotFound) {
			sess, err = s.store.Sessions.New(w)
		}
		if err != nil {
			htmlutil.ServerErrorResponse(w, r, err, s.logger)
			return
		}

		before := maps.Clone(sess.Data)
		beforeUser := sessionUserID(sess)

		next.ServeHTTP(w, session.SetContext(r, sess))

		if sess.Dropped || (maps.Equal(before, sess.Data) && beforeUser == sessionUserID(sess)) {
			return
		}

		if err = s.store.Sessions.Save(r.Context(), sess); err != nil {
			s.logger.Error("session not saved", "err", err, "path", r.URL.Path)
		}
	})
}
