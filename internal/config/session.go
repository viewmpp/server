package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Session struct {
	SessionLifetime time.Duration
}

func (cfg *Config) loadSession() {
	flag.DurationVar(&cfg.SessionLifetime, "session-lifetime", env.GetDuration("SESSION_LIFETIME", 12*time.Hour), "session lifetime")
}
