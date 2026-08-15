package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Mailer struct {
	Resend
	VerificationTTL time.Duration
	VerificationRC  time.Duration
}

type Resend struct {
	APIKey string
	Sender string
}

func (cfg *Config) loadMailer() {
	flag.StringVar(&cfg.APIKey, "resend-api-key", env.GetString("RESEND_API_KEY", ""), "resend api key")
	flag.StringVar(&cfg.Sender, "resend-sender", env.GetString("RESEND_SENDER", ""), "resend mail sender")
	flag.DurationVar(&cfg.VerificationTTL, "verification-ttl", env.GetDuration("VERIFICATION_TTL", 30*time.Minute), "verification time to live")
	flag.DurationVar(&cfg.VerificationRC, "verification-resend-cooldown", env.GetDuration("VERIFICATION_RESEND_COOLDOWN", 1*time.Minute),
		"mail verification resend cooldown")
}
