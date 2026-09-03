package config

import (
	"flag"
	"server/internal/env"
	"time"
)

type Mailer struct {
	Resend
	MailerVerificationTTL time.Duration
	MailerVerificationRC  time.Duration
}

type Resend struct {
	ResendAPIKey string
	ResendSender string
}

func (cfg *Config) loadMailer() {
	flag.StringVar(&cfg.ResendAPIKey, "resend-api-key", env.GetString("RESEND_API_KEY", ""), "resend api key")
	flag.StringVar(&cfg.ResendSender, "resend-sender", env.GetString("RESEND_SENDER", ""), "resend mail sender")
	flag.DurationVar(&cfg.MailerVerificationTTL, "verification-ttl", env.GetDuration("VERIFICATION_TTL", 30*time.Minute), "verification time to live")
	flag.DurationVar(&cfg.MailerVerificationRC, "verification-resend-cooldown", env.GetDuration("VERIFICATION_RESEND_COOLDOWN", 1*time.Minute),
		"mail verification resend cooldown")
}
