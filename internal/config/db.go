package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type DB struct {
	DBDSN        string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  time.Duration
}

func (cfg *Config) loadDB() {
	flag.StringVar(&cfg.DBDSN, "dsn", env.GetString("DB_DSN", ""),
		"postgres data source name")
	flag.IntVar(&cfg.MaxOpenConns, "max-open-conns", env.GetInt("DB_MAX_OPEN_CONNS", 25), "postgres max open connections")
	flag.IntVar(&cfg.MaxIdleConns, "max-idle-conns", env.GetInt("DB_MAX_IDLE_CONNS", 25), "postgres max idle connections")
	flag.DurationVar(&cfg.MaxIdleTime, "max-idle-time", env.GetDuration("DB_MAX_IDLE_TIME", 15*time.Minute), "postgres max idle time")
}
