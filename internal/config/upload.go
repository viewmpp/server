package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Upload struct {
	Client
	UploadLimit  int
	UploadWindow time.Duration
}

type Client struct {
	ClientTimeout time.Duration
}

func (cfg *Config) loadUpload() {
	flag.DurationVar(&cfg.ClientTimeout, "client-timeout", env.GetDuration("CLIENT_TIMEOUT", 30*time.Second),
		"client time limit for requests")
	flag.IntVar(&cfg.UploadLimit, "upload-limit", env.GetInt("UPLOAD_LIMIT", 10),
		"uploads allowed per client within the upload window")
	flag.DurationVar(&cfg.UploadWindow, "upload-window", env.GetDuration("UPLOAD_WINDOW", time.Minute),
		"window for the upload limit")
}
