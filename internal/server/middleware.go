package server

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"server/internal/clientip"
	"server/internal/htmlutil"
	"server/internal/jsonutil"
	"server/internal/ratelimit"
	"server/internal/safelog"
	"server/internal/session"
	"server/internal/user"
	"strconv"
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

type noStoreWriter struct {
	http.ResponseWriter
}

func (w *noStoreWriter) WriteHeader(code int) {
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(code)
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
			slog.String("uri", safelog.URI(r.RequestURI)),
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

func (s *Server) noStore(next http.Handler) http.Handler {
	if s.cfg.Env != "dev" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
	})
}

func (s *Server) throttle(limiter *ratelimit.Limiter, prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := prefix + clientip.From(r)

		if !limiter.Take(key) {
			s.logger.Warn("read throttled", "limit", safelog.Key(key))
			s.tooManyRequests(w, r, limiter.Window())
			return
		}

		next(w, r)
	}
}

func (s *Server) tooManyRequests(w http.ResponseWriter, r *http.Request, window time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))

	if strings.HasPrefix(r.URL.Path, "/api/") {
		jsonutil.TooManyRequestsResponse(w, "too many requests from this address, try again shortly")
		return
	}

	htmlutil.TooManyRequestsPage(w)
}

var policy = strings.Join([]string{
	"default-src 'self'",
	"base-uri 'none'",
	"object-src 'none'",
	"frame-ancestors 'none'",
	"form-action 'self'",
	"connect-src 'self'",
	"img-src 'self' data:",
	"font-src 'self' data: https://fonts.gstatic.com",
	"style-src 'self' 'unsafe-inline'",
	"script-src 'self' 'unsafe-inline'",
}, "; ")

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()

		header.Set("Content-Security-Policy", policy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if s.cfg.Env == "prod" {
			header.Set("Strict-Transport-Security", "max-age=31536000")
		}

		next.ServeHTTP(w, r)
	})
}

var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if safeMethods[r.Method] || s.fromOwnSite(r) {
			next.ServeHTTP(w, r)
			return
		}

		s.logger.Warn("cross site request refused", "method", r.Method, "uri", safelog.URI(r.URL.Path))

		if strings.HasPrefix(r.URL.Path, "/api/") {
			jsonutil.ForbiddenResponse(w, "this request did not come from the site")
			return
		}

		htmlutil.BadRequestPage(w, r, s.logger)
	})
}

func (s *Server) fromOwnSite(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = originOf(r.Header.Get("Referer"))
	}

	if origin == "" {
		return true
	}

	return origin == strings.TrimSuffix(s.cfg.BaseURL, "/")
}

func originOf(reference string) string {
	if reference == "" {
		return ""
	}

	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}

func (s *Server) clientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := s.resolver.Get(r)

		next.ServeHTTP(w, clientip.SetContext(r, ip))
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
			sess, err = s.store.Sessions.New(w, r)
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
