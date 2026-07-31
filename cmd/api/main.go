package main

import (
	"fmt"
	"log/slog"
	"os"
	"server/internal/config"
	"server/internal/htmlutil"
	"server/internal/server"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	templates, err := htmlutil.NewTemplates()
	if err != nil {
		return err
	}

	viewerHandler := viewer.NewHandler(logger, templates)

	var wg sync.WaitGroup

	return server.New(cfg, logger, viewerHandler, &wg).Serve()
}
