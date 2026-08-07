package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/config"
	"server/internal/htmlutil"
	"server/internal/mailer"
	"server/internal/parser"
	"server/internal/postgres"
	"server/internal/project"
	"server/internal/ratelimit"
	"server/internal/server"
	"server/internal/session"
	"server/internal/token"
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

	projectStore := project.NewStore(db)
	userStore := user.NewStore(db)
	sessions := session.NewStore(db, cfg.Env != "dev", logger)
	tokens := token.NewStore(db)
	mail := mailer.New(cfg.APIKey, cfg.Sender, templates, logger)

	client := &parser.Client{
		URL:  cfg.URL,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}

	viewerHandler := viewer.NewHandler(templates, logger)
	uploadHandler := upload.NewHandler(client, projectStore, logger)
	userHandler := user.NewHandler(userStore, sessions, limiter, tokens, mail, templates, &wg, logger)

	return server.New(cfg, viewerHandler, uploadHandler, userHandler, userStore, sessions, &wg, logger).Serve()
}
