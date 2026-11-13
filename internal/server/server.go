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
	"server/internal/billing"
	"server/internal/clientip"
	"server/internal/config"
	"server/internal/diagnostics"
	"server/internal/export"
	"server/internal/parser"
	"server/internal/postgres"
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

type Services struct {
	cfg    config.Config
	client *parser.Client
	pg     *postgres.Postgres
	str    *store.Store
	diag   *diagnostics.Server
	logger *slog.Logger
}

func NewServices(
	cfg config.Config,
	client *parser.Client,
	pg *postgres.Postgres,
	str *store.Store,
	diag *diagnostics.Server,
	logger *slog.Logger,
) Services {
	return Services{
		cfg:    cfg,
		client: client,
		pg:     pg,
		str:    str,
		diag:   diag,
		logger: logger,
	}
}

type Limits struct {
	resolver      *clientip.Resolver
	readLimits    *ratelimit.Limiter
	exportLimits  *ratelimit.Limiter
	addressLimits *ratelimit.Limiter
	noticeLimits  *ratelimit.Limiter
}

func NewLimits(
	resolver *clientip.Resolver,
	readLimits *ratelimit.Limiter,
	exportLimits *ratelimit.Limiter,
	addressLimits *ratelimit.Limiter,
	noticeLimits *ratelimit.Limiter,
) Limits {
	return Limits{
		resolver:      resolver,
		readLimits:    readLimits,
		exportLimits:  exportLimits,
		addressLimits: addressLimits,
		noticeLimits:  noticeLimits,
	}
}

type Handlers struct {
	viewerHandler  *viewer.Handler
	uploadHandler  *upload.Handler
	exportHandler  *export.Handler
	projectHandler *project.Handler
	userHandler    *user.Handler
	billingHandler *billing.Handler
}

func NewHandlers(
	viewerHandler *viewer.Handler,
	uploadHandler *upload.Handler,
	exportHandler *export.Handler,
	projectHandler *project.Handler,
	userHandler *user.Handler,
	billingHandler *billing.Handler,
) Handlers {
	return Handlers{
		viewerHandler:  viewerHandler,
		uploadHandler:  uploadHandler,
		exportHandler:  exportHandler,
		projectHandler: projectHandler,
		userHandler:    userHandler,
		billingHandler: billingHandler,
	}
}

type Server struct {
	Services
	Limits
	Handlers
	health healthCache
	wg     *sync.WaitGroup
}

const (
	healthy   = "OK"
	unhealthy = "DOWN"
)

const ThrottleNoticeWindow = time.Minute

func New(
	services Services,
	limits Limits,
	handlers Handlers,
	wg *sync.WaitGroup,
) *Server {
	return &Server{
		Services: services,
		Limits:   limits,
		Handlers: handlers,
		wg:       wg,
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

		appErr := srv.Shutdown(ctx)
		diagErr := s.diag.Shutdown(ctx)

		s.logger.Info("completing background tasks", "addr", srv.Addr)

		s.wg.Wait()

		s.logger.Info("stopped server")

		shutdownError <- errors.Join(appErr, diagErr)
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
