package config

import (
	"flag"
	"server/internal/env"
)

type Config struct {
	Port int
	Env  string
	Parser
}

type Parser struct {
	URL string
}

func Load() Config {

	var cfg Config

	flag.IntVar(&cfg.Port, "port", env.GetInt("PORT", 4000), "server port")
	flag.StringVar(&cfg.Env, "env", env.GetString("ENV", "dev"), "environment dev|stage|prod")
	flag.StringVar(&cfg.URL, "parser-url", env.GetString("PARSER_URL", "http://localhost:8080/parse"), "parser url")

	flag.Parse()

	return cfg
}
