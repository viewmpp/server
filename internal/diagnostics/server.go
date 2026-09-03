package diagnostics

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Server struct {
	srv    *http.Server
	logger *slog.Logger
}

func New(addr string, logger *slog.Logger) *Server {

	srv := &http.Server{
		Addr:              addr,
		Handler:           route(),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		WriteTimeout:      2 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	return &Server{
		srv:    srv,
		logger: logger,
	}
}

func (s *Server) ListenAndServe() {
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		s.logger.Error("failed to start diagnostics listener", "error", err)
		return
	}
	defer listener.Close()

	s.logger.Info("starting diagnostics server", "addr", listener.Addr())

	err = s.srv.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("diagnostics server failed", "error", err)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down the diagnostic server")
	return s.srv.Shutdown(ctx)
}
