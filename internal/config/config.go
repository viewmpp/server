package config

import (
	"flag"
	"server/internal/env"
)

type Config struct {
	Port int
	Env  string
	Parser
	DB
	Resend
}

type Parser struct {
	URL string
}

type DB struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  string
}

type Resend struct {
	APIKey string
	Sender string
}

func Load() Config {

	var cfg Config

	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.StringVar(&cfg.URL, "parser-url", env.GetString("PARSER_URL", "http://localhost:8080/parse"), "parser url")
	flag.StringVar(&cfg.DSN, "dsn", env.GetString("DB_DSN", ""),
		"postgres data source name")
	flag.IntVar(&cfg.MaxOpenConns, "max-open-conns", env.GetInt("DB_MAX_OPEN_CONNS", 25), "postgres max open connections")
	flag.IntVar(&cfg.MaxIdleConns, "max-idle-conns", env.GetInt("DB_MAX_IDLE_CONNS", 25), "postgres max idle connections")
	flag.StringVar(&cfg.MaxIdleTime, "max-idle-time", env.GetString("DB_MAX_IDLE_TIME", "15m"), "postgres max idle time")
	flag.StringVar(&cfg.APIKey, "resend-api-key", env.GetString("RESEND_API_KEY", ""), "resend api key")
	flag.StringVar(&cfg.Sender, "resend-sender", env.GetString("RESEND_SENDER", ""), "resend mail sender")

	flag.Parse()

	return cfg
}
