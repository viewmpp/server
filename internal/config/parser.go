package config

import (
	"flag"
	"server/internal/env"
)

type Parser struct {
	ParserURL string
}

func (cfg *Config) loadParser() {
	flag.StringVar(&cfg.ParserURL, "parser-url", env.GetString("PARSER_URL", "http://localhost:8080/parse"), "parser url")
}
