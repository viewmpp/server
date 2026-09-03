package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Project struct {
	ProjLenListLimit int
	ProjectLimit     int
	ProjectWindow    time.Duration
}

func (cfg *Config) loadProject() {
	flag.IntVar(&cfg.ProjLenListLimit, "project-list-limit", env.GetInt("PROJECT_LIST_LIMIT", 100), "project list limit")
	flag.IntVar(&cfg.ProjectLimit, "project-limit", env.GetInt("PROJECT_LIMIT", 10),
		"unlock attempts allowed per protected link within the project window")
	flag.DurationVar(&cfg.ProjectWindow, "project-window", env.GetDuration("PROJECT_WINDOW", time.Minute),
		"window for the unlock attempt limit")
}
