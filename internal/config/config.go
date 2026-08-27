package config

import (
	"flag"
	"fmt"
	"strings"
)

type Config struct {
	Application
	Upload
	Project
	Read
	User
	Parser
	DB
	Mailer
	Background
	Session
	Reset
}

func Load() Config {

	var cfg Config

	cfg.loadApplication()
	cfg.loadUpload()
	cfg.loadProject()
	cfg.loadRead()
	cfg.loadUser()
	cfg.loadParser()
	cfg.loadDB()
	cfg.loadMailer()
	cfg.loadBackground()
	cfg.loadSession()
	cfg.loadReset()

	flag.Parse()

	return cfg
}

func (cfg Config) Warnings() []string {
	var out []string

	if cfg.Env == "prod" && cfg.Proxies == 0 {
		out = append(out, "PROXIES is 0 in prod: if anything proxies this server, every visitor "+
			"resolves to the proxy address and one rate limit is shared by the whole site")
	}

	return out
}

func (cfg Config) Validate() error {
	if cfg.Env != "prod" {
		return nil
	}

	if cfg.BaseURL == "" || strings.Contains(cfg.BaseURL, "localhost") || strings.Contains(cfg.BaseURL, "127.0.0.1") {
		return fmt.Errorf("BASE_URL is %q in prod: it signs every canonical tag, the sitemap and every password reset link", cfg.BaseURL)
	}

	if !strings.HasPrefix(cfg.BaseURL, "https://") {
		return fmt.Errorf("BASE_URL is %q in prod: reset links and cookies must travel over https", cfg.BaseURL)
	}

	return nil
}
