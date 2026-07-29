package config

import (
	"flag"
	"server/internal/env"
)

type Config struct {
	Port int
	Env  string
}

func Load() Config {

	var cfg Config

	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")

	flag.Parse()

	return cfg
}
