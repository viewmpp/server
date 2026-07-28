package main

import (
	"fmt"
	"log/slog"
	"mpp-viewer-server/internal/config"
	"mpp-viewer-server/internal/server"
	"os"
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

	srv := server.NewServer(cfg, logger)

	return srv.Serve()
}
