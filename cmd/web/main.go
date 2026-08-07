package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"server/internal/config"
	"server/internal/htmlutil"
	"server/internal/parser"
	"server/internal/postgres"
	"server/internal/server"
	"server/internal/session"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		return err
	}

	viewerHandler := viewer.NewHandler(logger, templates)

	client := &parser.Client{
		URL:  cfg.URL,
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
	uploadHandler := upload.NewHandler(logger, client, nil)

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	sessions := session.NewStore(db, cfg.Env != "dev")

	userStore := user.NewStore(db)
	userHandler := user.NewHandler(logger, userStore, sessions, templates)

	var wg sync.WaitGroup

	return server.New(cfg, logger, viewerHandler, uploadHandler, userHandler, sessions, &wg).Serve()
}
