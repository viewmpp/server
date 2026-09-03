package config

import (
	"errors"
	"flag"
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

	cfg.fillSecretKey()

	return cfg
}

func (cfg *Config) Validate() error {
	switch {
	case cfg.Env != "prod":
		return nil
	case cfg.Proxies < 1:
		return errors.New("prod started with 0 proxies")
	case len(cfg.SecretKey) < MinSecretKeyLength:
		return errors.New("prod started without SECRET_KEY")
	case cfg.BaseURL == "" || strings.Contains(cfg.BaseURL, "localhost") || strings.Contains(cfg.BaseURL, "127.0.0.1"):
		return errors.New("")
	case !strings.HasPrefix(cfg.BaseURL, "https://"):
		return errors.New("")
	default:
		return nil
	}
}
