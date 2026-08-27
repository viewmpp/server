package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/background"
	"server/internal/clientip"
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

	if err := cfg.Validate(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	for _, warning := range cfg.Warnings() {
		logger.Warn(warning)
	}

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

	htmlutil.SetBaseURL(cfg.BaseURL)

	resolver, err := clientip.NewResolver(cfg.Proxies)
	if err != nil {
		return err
	}

	uploadLimiter := ratelimit.New(cfg.UploadLimit, cfg.UploadWindow)
	defer uploadLimiter.Close()

	projectLimiter := ratelimit.New(cfg.ProjectLimit, cfg.ProjectWindow)
	defer projectLimiter.Close()

	readLimiter := ratelimit.New(cfg.ReadLimit, cfg.ReadWindow)
	defer readLimiter.Close()

	exportLimiter := ratelimit.New(cfg.ExportLimit, cfg.ExportWindow)
	defer exportLimiter.Close()

	userLimiter := ratelimit.New(cfg.UserLimit, cfg.UserWindow)
	defer userLimiter.Close()

	s := store.New(db, cfg, logger)

	stop := make(chan struct{})
	defer close(stop)

	background.Sweep(stop, logger, "sessions", cfg.SweepRepetition, cfg.SweepTimeout, s.Sessions.DeleteExpired)
	background.Sweep(stop, logger, "tokens", cfg.SweepRepetition, cfg.SweepTimeout, s.Tokens.DeleteExpired)
	background.Sweep(stop, logger, "protected links", cfg.SweepRepetition, cfg.SweepTimeout, s.Projects.DemoteExpiredProtected)

	mail := mailer.New(cfg.Resend, templates, logger)

	viewerHandler := viewer.NewHandler(templates, cfg.BaseURL, logger)
	uploadHandler := upload.NewHandler(client, s.Projects, uploadLimiter, logger)
	exportHandler := export.NewHandler(uploadLimiter, logger)
	projectHandler := project.NewHandler(s.Projects, cfg.BaseURL, projectLimiter, cfg.LenListLimit, templates, logger)
	userHandler := user.NewHandler(s.Users, s.Projects, s.Tokens, s.Sessions, userLimiter, mail, cfg.VerificationTTL, cfg.VerificationRC, cfg.ResetTTL, cfg.BaseURL, cfg.EarlyAccessSeats, cfg.EarlyAccessPeriod, templates, &wg, logger)

	return server.New(cfg, resolver, readLimiter, exportLimiter, viewerHandler, uploadHandler, exportHandler, projectHandler, userHandler, s, &wg, logger).Serve()
}
