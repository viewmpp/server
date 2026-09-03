package diagnostics

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"server/internal/config"
	"time"
)

type Server struct {
	srv    *http.Server
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Server {

	srv := &http.Server{
		Addr:              cfg.DiagAddr,
		Handler:           route(),
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		WriteTimeout:      2 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	return &Server{
		srv:    srv,
		cfg:    cfg,
		logger: logger,
	}
}

func (s *Server) ListenAndServe() {
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		s.logger.Error("something went wrong while establishing tcp connection")
		return
	}
	defer listener.Close()

	s.logger.Info("starting diagnostic server listener", "addr", listener.Addr())

	err = s.srv.Serve(listener)
	if err != nil {
		s.logger.Error("diagnostics server failed", "error", err)
		return
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down the diagnostic server")
	return s.srv.Shutdown(ctx)
}
