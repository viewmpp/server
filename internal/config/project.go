package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Project struct {
	LenListLimit  int
	ProjectLimit  int
	ProjectWindow time.Duration
}

func (cfg *Config) loadProject() {
	flag.IntVar(&cfg.LenListLimit, "project-list-limit", env.GetInt("PROJECT_LIST_LIMIT", 100), "project list limit")
	flag.IntVar(&cfg.ProjectLimit, "project-limit", env.GetInt("PROJECT_LIMIT", 10), "")
	flag.DurationVar(&cfg.ProjectWindow, "project-window", env.GetDuration("PROJECT_WINDOW", time.Minute), "")
}
