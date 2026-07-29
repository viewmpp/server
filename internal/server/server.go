package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"server/internal/config"
	"server/internal/viewer"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	cfg           config.Config
	logger        *slog.Logger
	viewerHandler *viewer.Handler
	wg            *sync.WaitGroup
}

func New(cfg config.Config, logger *slog.Logger, viewerHandler *viewer.Handler, wg *sync.WaitGroup) *Server {
	return &Server{
		cfg:           cfg,
		logger:        logger,
		viewerHandler: viewerHandler,
		wg:            wg,
	}
}

func (s *Server) Serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.Port),
		Handler:           s.routes(),
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelError),
		WriteTimeout:      2 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	s.logger.Info("starting server", "port", s.cfg.Port, "env", s.cfg.Env)

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit

		s.logger.Info("shutting down the server", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
			return
		}

		s.logger.Info("completing background tasks", "addr", srv.Addr)

		s.wg.Wait()

		s.logger.Info("stopped server")

		shutdownError <- nil
	}()

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-shutdownError
}
