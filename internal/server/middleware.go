package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				jsonutil.ServerErrorResponse(w, r, fmt.Errorf("%v", err), s.logger)
			}
		}()

		next.ServeHTTP(w, r)
	})
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
			slog.Duration("duration", time.Since(starts))}

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
