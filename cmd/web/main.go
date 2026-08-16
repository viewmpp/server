package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/background"
	"server/internal/config"
	"server/internal/export"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/parser"
	"server/internal/postgres"
	"server/internal/project"
	"server/internal/ratelimit"
	"server/internal/server"
	"server/internal/store"
	"server/internal/upload"
	"server/internal/user"
	"server/internal/viewer"
	"sync"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	var wg sync.WaitGroup

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	client := &parser.Client{
		URL:  cfg.ParserURL,
		HTTP: &http.Client{Timeout: cfg.ClientTimeout},
	}

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		return err
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	uploadLimiter, err := ratelimit.New(cfg.UploadLimit, cfg.UploadWindow, cfg.Proxies)
	if err != nil {
		return err
	}
	defer uploadLimiter.Close()

	projectLimiter, err := ratelimit.New(cfg.ProjectLimit, cfg.ProjectWindow, cfg.Proxies)
	if err != nil {
		return err
	}
	defer projectLimiter.Close()

	userLimiter, err := ratelimit.New(cfg.UserLimit, cfg.UserWindow, cfg.Proxies)
	if err != nil {
		return err
	}
	defer userLimiter.Close()

	s := store.New(db, cfg, logger)

	stop := make(chan struct{})
	defer close(stop)

	background.Sweep(stop, logger, "sessions", cfg.SweepRepetition, cfg.SweepTimeout, s.Sessions.DeleteExpired)
	background.Sweep(stop, logger, "tokens", cfg.SweepRepetition, cfg.SweepTimeout, s.Tokens.DeleteExpired)

	mail := mailer.New(cfg.Resend, templates, logger)

	viewerHandler := viewer.NewHandler(templates, cfg.BaseURL, logger)
	uploadHandler := upload.NewHandler(client, s.Projects, uploadLimiter, logger)
	exportHandler := export.NewHandler(uploadLimiter, logger)
	projectHandler := project.NewHandler(s.Projects, cfg.BaseURL, userLimiter, cfg.LenListLimit, templates, logger)
	userHandler := user.NewHandler(s.Users, s.Tokens, s.Sessions, userLimiter, mail, cfg.VerificationTTL, cfg.VerificationRC, cfg.ResetTTL, cfg.BaseURL, cfg.EarlyAccessSeats, templates, &wg, logger)

	return server.New(cfg, viewerHandler, uploadHandler, exportHandler, projectHandler, userHandler, s, &wg, logger).Serve()
}
