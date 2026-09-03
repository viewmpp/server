package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Reset struct {
	ResetTTL time.Duration
}

func (cfg *Config) loadReset() {
	flag.DurationVar(&cfg.ResetTTL, "reset-ttl", env.GetDuration("RESET_TTL", time.Hour), "password reset link lifetime")
}
