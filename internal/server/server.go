package server

import (
	"fmt"
	"log/slog"
	"mpp-viewer-server/internal/config"
	"mpp-viewer-server/internal/viewer"
	"net/http"
	"time"
)

type Server struct {
	cfg           config.Config
	logger        *slog.Logger
	viewerHandler *viewer.Handler
}

func New(cfg config.Config, logger *slog.Logger, viewerHandler *viewer.Handler) *Server {
	return &Server{
		cfg:           cfg,
		logger:        logger,
		viewerHandler: viewerHandler,
	}
}

func (s *Server) Serve() error {

	srv := http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.Port),
		Handler:           s.routes(),
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelError),
		WriteTimeout:      2 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	s.logger.Info("starting server", "port", s.cfg.Port, "env", s.cfg.Env)

	return srv.ListenAndServe()
}
