package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"server/internal/env"
	"time"
)

const MinSecretKeyLength = 32

type Session struct {
	SessionLifetime time.Duration
	SecretKey       string
}

func (cfg *Config) loadSession() {
	flag.DurationVar(&cfg.SessionLifetime, "session-lifetime", env.GetDuration("SESSION_LIFETIME", 12*time.Hour), "session lifetime")
	flag.StringVar(&cfg.SecretKey, "secret-key", env.GetString("SECRET_KEY", ""),
		"key signing csrf tokens; required in prod, generated on every start elsewhere")
}

func (cfg *Config) fillSecretKey() {
	if cfg.SecretKey != "" || cfg.Env == "prod" {
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return
	}

	cfg.SecretKey = hex.EncodeToString(raw)
}
