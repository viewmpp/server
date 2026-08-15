package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type User struct {
	UserLimit  int
	UserWindow time.Duration
}

func (cfg *Config) loadUser() {
	flag.IntVar(&cfg.UserLimit, "user-limit", env.GetInt("USER_LIMIT", 10), "")
	flag.DurationVar(&cfg.UserWindow, "user-window", env.GetDuration("USER_WINDOW", time.Minute), "")
}
