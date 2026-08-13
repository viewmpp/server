package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/background"
	"server/internal/config"
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
	"time"
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

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		return err
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	limiter := ratelimit.New(5, time.Minute)
	defer limiter.Close()

	s := store.New(db, cfg, logger)

	stop := make(chan struct{})
	defer close(stop)

	bgRepetition, err := time.ParseDuration(cfg.Repetition)
	if err != nil {
		return err
	}

	bgTimeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return err
	}

	background.Sweep(stop, logger, "sessions", bgRepetition, bgTimeout, s.Sessions.DeleteExpired)
	background.Sweep(stop, logger, "tokens", bgRepetition, bgTimeout, s.Tokens.DeleteExpired)

	vttl, err := time.ParseDuration(cfg.VerificationTTL)
	if err != nil {
		return err
	}
	vrc, err := time.ParseDuration(cfg.VerificationRC)
	if err != nil {
		return err
	}
	mail := mailer.New(cfg.Resend, templates, logger)

	client := &parser.Client{
		URL:  cfg.URL,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}

	viewerHandler := viewer.NewHandler(templates, logger)
	uploadHandler := upload.NewHandler(client, s.Projects, logger)
	projectHandler := project.NewHandler(s.Projects, templates, logger)
	userHandler := user.NewHandler(s.Users, s.Tokens, s.Sessions, limiter, mail, vttl, vrc, templates, &wg, logger)

	return server.New(cfg, viewerHandler, uploadHandler, projectHandler, userHandler, s, &wg, logger).Serve()
}
