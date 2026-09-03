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
	"server/internal/clientip"
	"server/internal/config"
	"server/internal/diagnostics"
	"server/internal/export"
	"server/internal/project"
	"server/internal/ratelimit"
	"server/internal/store"
	"server/internal/upload"
	"server/internal/user"
	"server/internal/viewer"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	cfg            config.Config
	diagnostics    *diagnostics.Server
	resolver       *clientip.Resolver
	readLimiter    *ratelimit.Limiter
	exportLimiter  *ratelimit.Limiter
	addressLimiter *ratelimit.Limiter
	throttleNotice *ratelimit.Limiter
	viewerHandler  *viewer.Handler
	uploadHandler  *upload.Handler
	exportHandler  *export.Handler
	projectHandler *project.Handler
	userHandler    *user.Handler
	store          *store.Store
	wg             *sync.WaitGroup
	logger         *slog.Logger
}

const throttleNoticeWindow = time.Minute

func New(
	cfg config.Config,
	diagnostics *diagnostics.Server,
	resolver *clientip.Resolver,
	readLimiter *ratelimit.Limiter,
	exportLimiter *ratelimit.Limiter,
	addressLimiter *ratelimit.Limiter,
	viewerHandler *viewer.Handler,
	uploadHandler *upload.Handler,
	exportHandler *export.Handler,
	projectHandler *project.Handler,
	userHandler *user.Handler,
	store *store.Store,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *Server {
	return &Server{
		cfg:            cfg,
		diagnostics:    diagnostics,
		resolver:       resolver,
		readLimiter:    readLimiter,
		exportLimiter:  exportLimiter,
		addressLimiter: addressLimiter,
		throttleNotice: ratelimit.New(1, throttleNoticeWindow),
		viewerHandler:  viewerHandler,
		uploadHandler:  uploadHandler,
		exportHandler:  exportHandler,
		projectHandler: projectHandler,
		userHandler:    userHandler,
		store:          store,
		wg:             wg,
		logger:         logger,
	}
}

func (s *Server) Serve() error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.AppPort),
		Handler:           s.routes(),
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelError),
		WriteTimeout:      2 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	shutdownError := make(chan error, 1)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit

		s.logger.Info("shutting down the server", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		diagErr := s.diagnostics.Shutdown(ctx)
		appErr := srv.Shutdown(ctx)

		shutdownError <- errors.Join(diagErr, appErr)

		s.throttleNotice.Close()

		s.logger.Info("completing background tasks", "addr", srv.Addr)

		s.wg.Wait()

		s.logger.Info("stopped server")

		shutdownError <- nil
	}()

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	s.logger.Info("starting server", "addr", ln.Addr(), "env", s.cfg.AppEnv)

	err = srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-shutdownError
}
