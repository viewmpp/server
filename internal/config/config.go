package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Config struct {
	Port             int
	Env              string
	Proxies          int
	EarlyAccessSeats int
	Project
	Parser
	DB
	Mailer
	BGSweep
	Session
	Reset
}

type Project struct {
	ListLimit int
}

type Parser struct {
	URL string
}

type DB struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  time.Duration
}

type Mailer struct {
	Resend
	VerificationTTL time.Duration
	VerificationRC  time.Duration
}

type Resend struct {
	APIKey string
	Sender string
}

type BGSweep struct {
	Repetition time.Duration
	Timeout    time.Duration
}

type Reset struct {
	ResetTTL time.Duration
	BaseURL  string
}

type Session struct {
	Lifetime time.Duration
}

func Load() Config {

	var cfg Config

	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.IntVar(&cfg.EarlyAccessSeats, "early-access-seats", env.GetInt("EARLY_ACCESS_SEATS", 100),
		"how many users may claim Pro for free; 0 closes early access")
	flag.DurationVar(&cfg.ResetTTL, "reset-ttl", env.GetDuration("RESET_TTL", time.Hour), "password reset link lifetime")
	flag.StringVar(&cfg.BaseURL, "base-url", env.GetString("BASE_URL", "http://localhost:4000"),
		"public address used in emailed links")
	flag.IntVar(&cfg.Proxies, "proxies", env.GetInt("PROXIES", 0),
		"number of trusted reverse proxies in front of the server; 0 means none")
	flag.StringVar(&cfg.URL, "parser-url", env.GetString("PARSER_URL", "http://localhost:8080/parse"), "parser url")
	flag.StringVar(&cfg.DSN, "dsn", env.GetString("DB_DSN", ""),
		"postgres data source name")
	flag.IntVar(&cfg.MaxOpenConns, "max-open-conns", env.GetInt("DB_MAX_OPEN_CONNS", 25), "postgres max open connections")
	flag.IntVar(&cfg.MaxIdleConns, "max-idle-conns", env.GetInt("DB_MAX_IDLE_CONNS", 25), "postgres max idle connections")
	flag.DurationVar(&cfg.MaxIdleTime, "max-idle-time", env.GetDuration("DB_MAX_IDLE_TIME", 15*time.Minute), "postgres max idle time")
	flag.StringVar(&cfg.APIKey, "resend-api-key", env.GetString("RESEND_API_KEY", ""), "resend api key")
	flag.StringVar(&cfg.Sender, "resend-sender", env.GetString("RESEND_SENDER", ""), "resend mail sender")
	flag.DurationVar(&cfg.VerificationTTL, "verification-ttl", env.GetDuration("VERIFICATION_TTL", 30*time.Minute), "verification time to live")
	flag.DurationVar(&cfg.VerificationRC, "verification-resend-cooldown", env.GetDuration("VERIFICATION_RESEND_COOLDOWN", 1*time.Minute),
		"mail verification resend cooldown")
	flag.DurationVar(&cfg.Repetition, "background-sweep-repetition", env.GetDuration("BACKGROUND_SWEEP_REPETITION", 1*time.Hour),
		"background goroutine sweep repetition")
	flag.DurationVar(&cfg.Timeout, "background-sweep-timeout", env.GetDuration("BACKGROUND_SWEEP_TIMEOUT", 30*time.Second),
		"background goroutine sweep timeout")
	flag.DurationVar(&cfg.Lifetime, "session-lifetime", env.GetDuration("SESSION_LIFETIME", 12*time.Hour), "session lifetime")
	flag.IntVar(&cfg.ListLimit, "list-limit", env.GetInt("LIST_LIMIT", 100), "project list limit")

	flag.Parse()

	return cfg
}
