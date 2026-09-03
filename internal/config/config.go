package config

import (
	"errors"
	"flag"
	"strings"
)

type Config struct {
	Application
	Diagnostics
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
	cfg.loadDiagnostics()
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
	case cfg.AppEnv != "prod":
		return nil
	case cfg.AppProxies < 1:
		return errors.New("prod started with 0 proxies")
	case len(cfg.SessionSecretKey) < MinSecretKeyLength:
		return errors.New("prod started without SECRET_KEY")
	case cfg.AppBaseURL == "" ||
		strings.Contains(cfg.AppBaseURL, "localhost") ||
		strings.Contains(cfg.AppBaseURL, "127.0.0.1"):
		return errors.New("prod server started with empty or localhost base url")
	case !strings.HasPrefix(cfg.AppBaseURL, "https://"):
		return errors.New("prod started with ")
	case cfg.DiagAddr == "" ||
		strings.Contains(cfg.DiagAddr, "localhost") ||
		strings.Contains(cfg.DiagAddr, "127.0.0.1") ||
		strings.Contains(cfg.DiagAddr, "4000") ||
		strings.Contains(cfg.DiagAddr, "5432") ||
		strings.Contains(cfg.DiagAddr, "8080"):
		return errors.New("prod diagnostic server started with empty or localhost address, or using illegal ports (:4000, :5432, :8080)")
	default:
		return nil
	}
}
