package config

import (
	"flag"
	"log/slog"
	"server/internal/env"
	"strings"
	"time"
)

type Application struct {
	AppBaseURL           string
	AppPort              int
	AppEnv               string
	AppLogLevel          string
	AppProxies           int
	AppEarlyAccessSeats  int
	AppEarlyAccessPeriod time.Duration
}

func (cfg *Config) loadApplication() {
	flag.StringVar(&cfg.AppBaseURL, "base-url", env.GetString("BASE_URL", "http://localhost:4000"),
		"public address used in emailed links")
	flag.IntVar(&cfg.AppPort, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.AppEnv, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.StringVar(&cfg.AppLogLevel, "log-level", env.GetString("LOG_LEVEL", "info"),
		"lowest level written to the log: debug|info|warn|error")
	flag.IntVar(&cfg.AppEarlyAccessSeats, "early-access-seats", env.GetInt("EARLY_ACCESS_SEATS", 100),
		"how many users may claim Pro for free; 0 closes early access")
	flag.DurationVar(&cfg.AppEarlyAccessPeriod, "early-access-period", env.GetDuration("EARLY_ACCESS_PERIOD", 90*24*time.Hour),
		"how long an early access grant lasts before it has to be renewed")
	flag.IntVar(&cfg.AppProxies, "proxies", env.GetInt("PROXIES", 0),
		"number of trusted reverse proxies in front of the server; 0 means none")
}

func (cfg *Config) Level() slog.Level {
	switch strings.ToLower(cfg.AppLogLevel) {
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
