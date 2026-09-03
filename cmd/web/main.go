package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/background"
	"server/internal/clientip"
	"server/internal/config"
	"server/internal/diagnostics"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Level()}))

	diag := diagnostics.New(&cfg, logger)

	go diag.ListenAndServe()

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

	htmlutil.SetBaseURL(cfg.AppBaseURL)

	resolver, err := clientip.NewResolver(cfg.AppProxies)
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

	addressLimiter := ratelimit.New(cfg.AddressLimit, cfg.AddressWindow)
	defer addressLimiter.Close()

	userLimiter := ratelimit.New(cfg.UserLimit, cfg.UserWindow)
	defer userLimiter.Close()

	s := store.New(db, cfg, logger)

	stop := make(chan struct{})
	defer close(stop)

	background.Sweep(stop, logger, "sessions", cfg.BGSweepRepetition, cfg.BGSweepTimeout, s.Sessions.DeleteExpired)
	background.Sweep(stop, logger, "tokens", cfg.BGSweepRepetition, cfg.BGSweepTimeout, s.Tokens.DeleteExpired)
	background.Sweep(stop, logger, "protected links", cfg.BGSweepRepetition, cfg.BGSweepTimeout, s.Projects.DemoteExpiredProtected)

	mail := mailer.New(cfg.Resend, templates, logger, cfg.AppEnv == "prod")

	viewerHandler := viewer.NewHandler(templates, cfg.AppBaseURL, logger)
	uploadHandler := upload.NewHandler(client, s.Projects, uploadLimiter, logger)
	exportHandler := export.NewHandler(uploadLimiter, logger)
	projectHandler := project.NewHandler(s.Projects, cfg.AppBaseURL, projectLimiter, cfg.ProjLenListLimit, templates, logger)
	userHandler := user.NewHandler(s.Users, s.Projects, s.Tokens, s.Sessions, userLimiter, mail,
		cfg.MailerVerificationTTL, cfg.MailerVerificationRC, cfg.ResetTTL, cfg.AppBaseURL, cfg.AppEarlyAccessSeats, cfg.AppEarlyAccessPeriod, templates, &wg, logger)

	return server.New(cfg, diag, resolver, readLimiter, exportLimiter, addressLimiter,
		viewerHandler, uploadHandler, exportHandler, projectHandler, userHandler, s, &wg, logger).Serve()
}
