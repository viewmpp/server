package config

import (
	"flag"
	"log/slog"
	"server/internal/env"
	"strings"
	"time"
)

type Application struct {
	Port              int
	Env               string
	LogLevel          string
	Proxies           int
	EarlyAccessSeats  int
	EarlyAccessPeriod time.Duration
}

func (cfg *Config) loadApplication() {
	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.StringVar(&cfg.LogLevel, "log-level", env.GetString("LOG_LEVEL", "info"),
		"lowest level written to the log: debug|info|warn|error")
	flag.IntVar(&cfg.EarlyAccessSeats, "early-access-seats", env.GetInt("EARLY_ACCESS_SEATS", 100),
		"how many users may claim Pro for free; 0 closes early access")
	flag.DurationVar(&cfg.EarlyAccessPeriod, "early-access-period", env.GetDuration("EARLY_ACCESS_PERIOD", 90*24*time.Hour),
		"how long an early access grant lasts before it has to be renewed")
	flag.IntVar(&cfg.Proxies, "proxies", env.GetInt("PROXIES", 0),
		"number of trusted reverse proxies in front of the server; 0 means none")
}

func (cfg *Config) Level() slog.Level {
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
