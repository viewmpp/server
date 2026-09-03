package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Background struct {
	BGSweepRepetition time.Duration
	BGSweepTimeout    time.Duration
}

func (cfg *Config) loadBackground() {
	flag.DurationVar(&cfg.BGSweepRepetition, "background-sweep-repetition", env.GetDuration("BACKGROUND_SWEEP_REPETITION", 1*time.Hour),
		"background goroutine sweep repetition")
	flag.DurationVar(&cfg.BGSweepTimeout, "background-sweep-timeout", env.GetDuration("BACKGROUND_SWEEP_TIMEOUT", 30*time.Second),
		"background goroutine sweep timeout")
}
