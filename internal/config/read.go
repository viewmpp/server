package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Read struct {
	ReadLimit    int
	ReadWindow   time.Duration
	ExportLimit  int
	ExportWindow time.Duration

	AddressLimit  int
	AddressWindow time.Duration
}

func (cfg *Config) loadRead() {
	flag.IntVar(&cfg.ReadLimit, "read-limit", env.GetInt("READ_LIMIT", 60),
		"shared plans a single address may open within the read window")
	flag.DurationVar(&cfg.ReadWindow, "read-window", env.GetDuration("READ_WINDOW", time.Minute),
		"window for the read limit")
	flag.IntVar(&cfg.ExportLimit, "export-limit", env.GetInt("EXPORT_LIMIT", 10),
		"spreadsheets a single address may build from saved plans within the export window")
	flag.DurationVar(&cfg.ExportWindow, "export-window", env.GetDuration("EXPORT_WINDOW", time.Minute),
		"window for the export limit")
	flag.IntVar(&cfg.AddressLimit, "address-limit", env.GetInt("ADDRESS_LIMIT", 900),
		"reads a single address may make within the address window, whoever the visitors behind it are")
	flag.DurationVar(&cfg.AddressWindow, "address-window", env.GetDuration("ADDRESS_WINDOW", time.Minute),
		"window for the address limit")
}
