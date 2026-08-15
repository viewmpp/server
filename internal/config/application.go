package config

import (
	"flag"
	"server/internal/env"
)

type Application struct {
	Port             int
	Env              string
	Proxies          int
	EarlyAccessSeats int
}

func (cfg *Config) loadApplication() {
	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.IntVar(&cfg.EarlyAccessSeats, "early-access-seats", env.GetInt("EARLY_ACCESS_SEATS", 100),
		"how many users may claim Pro for free; 0 closes early access")
	flag.IntVar(&cfg.Proxies, "proxies", env.GetInt("PROXIES", 0),
		"number of trusted reverse proxies in front of the server; 0 means none")
}
