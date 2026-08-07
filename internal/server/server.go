package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"server/internal/config"
	"server/internal/session"
	"server/internal/upload"
	"server/internal/user"
	"server/internal/viewer"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	cfg           config.Config
	viewerHandler *viewer.Handler
	uploadHandler *upload.Handler
	userHandler   *user.Handler
	userStore     *user.Store
	sessions      *session.Store
	wg            *sync.WaitGroup
	logger        *slog.Logger
}

func New(
	cfg config.Config,
	viewerHandler *viewer.Handler,
	uploadHandler *upload.Handler,
	userHandler *user.Handler,
	userStore *user.Store,
	sessions *session.Store,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *Server {
	return &Server{
		cfg:           cfg,
		viewerHandler: viewerHandler,
		uploadHandler: uploadHandler,
		userHandler:   userHandler,
		userStore:     userStore,
		sessions:      sessions,
		wg:            wg,
		logger:        logger,
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

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	s.logger.Info("starting server", "addr", ln.Addr(), "env", s.cfg.Env)

	err = srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-shutdownError
}
