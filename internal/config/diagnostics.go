package config

import (
	"flag"
	"server/internal/env"
)

type Diagnostics struct {
	DiagAddr string
}

func (cfg *Config) loadDiagnostics() {
	flag.StringVar(&cfg.DiagAddr, "diagnostic-server-address", env.GetString("DIAGNOSTIC_SERVER_ADDRESS", "127.0.0.1:6060"),
		"diagnostic server address")
}
