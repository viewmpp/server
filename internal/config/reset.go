package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Reset struct {
	ResetTTL time.Duration
	BaseURL  string
}

func (cfg *Config) loadReset() {
	flag.DurationVar(&cfg.ResetTTL, "reset-ttl", env.GetDuration("RESET_TTL", time.Hour), "password reset link lifetime")
	flag.StringVar(&cfg.BaseURL, "base-url", env.GetString("BASE_URL", "http://localhost:4000"),
		"public address used in emailed links")
}
