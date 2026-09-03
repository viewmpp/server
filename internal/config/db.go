package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type DB struct {
	DBDSN          string
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBMaxIdleTime  time.Duration
}

func (cfg *Config) loadDB() {
	flag.StringVar(&cfg.DBDSN, "dsn", env.GetString("DB_DSN", ""),
		"postgres data source name")
	flag.IntVar(&cfg.DBMaxOpenConns, "max-open-conns", env.GetInt("DB_MAX_OPEN_CONNS", 25), "postgres max open connections")
	flag.IntVar(&cfg.DBMaxIdleConns, "max-idle-conns", env.GetInt("DB_MAX_IDLE_CONNS", 25), "postgres max idle connections")
	flag.DurationVar(&cfg.DBMaxIdleTime, "max-idle-time", env.GetDuration("DB_MAX_IDLE_TIME", 15*time.Minute), "postgres max idle time")
}
