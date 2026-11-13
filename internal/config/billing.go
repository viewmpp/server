package config

import (
	"errors"
	"flag"
	"fmt"
	"server/internal/env"
	"strconv"
	"strings"
	"time"
)

type Billing struct {
	BillingAPIKey         string
	BillingWebhookSecret  string
	BillingStoreID        string
	BillingVariantMonthly string
	BillingVariantYearly  string
	BillingTestModeRaw    string
	BillingTestMode       bool
	BillingTestUsersRaw   string
	BillingTestUsers      []int64
	BillingTrialPeriod    time.Duration

	billingProblem error
}

func (cfg *Config) loadBilling() {
	flag.StringVar(&cfg.BillingAPIKey, "lemonsqueezy-api-key", env.GetString("LEMONSQUEEZY_API_KEY", ""), "lemon squeezy api key")
	flag.StringVar(&cfg.BillingWebhookSecret, "lemonsqueezy-webhook-secret", env.GetString("LEMONSQUEEZY_WEBHOOK_SECRET", ""),
		"secret lemon squeezy signs webhooks with")
	flag.StringVar(&cfg.BillingStoreID, "lemonsqueezy-store-id", env.GetString("LEMONSQUEEZY_STORE_ID", ""), "lemon squeezy store id")
	flag.StringVar(&cfg.BillingVariantMonthly, "lemonsqueezy-variant-monthly", env.GetString("LEMONSQUEEZY_VARIANT_MONTHLY", ""),
		"lemon squeezy variant id of the monthly plan")
	flag.StringVar(&cfg.BillingVariantYearly, "lemonsqueezy-variant-yearly", env.GetString("LEMONSQUEEZY_VARIANT_YEARLY", ""),
		"lemon squeezy variant id of the yearly plan")
	flag.StringVar(&cfg.BillingTestModeRaw, "lemonsqueezy-test-mode", env.GetString("LEMONSQUEEZY_TEST_MODE", ""),
		"true for lemon squeezy test mode, false for live payments")
	flag.StringVar(&cfg.BillingTestUsersRaw, "lemonsqueezy-test-users", env.GetString("LEMONSQUEEZY_TEST_USERS", ""),
		"comma separated account ids that see payments while in test mode")
	flag.DurationVar(&cfg.BillingTrialPeriod, "trial-period", env.GetDuration("TRIAL_PERIOD", 7*24*time.Hour),
		"length of the free pro trial once payments are offered")
}

func (b *Billing) fillBilling() {
	b.BillingTestMode = b.BillingTestModeRaw == "true"
	b.BillingTestUsers, b.billingProblem = parseIDs(b.BillingTestUsersRaw)
}

func parseIDs(raw string) ([]int64, error) {
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("LEMONSQUEEZY_TEST_USERS holds %q, which is not an account id", part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (b Billing) fields() []string {
	return []string{b.BillingAPIKey, b.BillingWebhookSecret, b.BillingStoreID, b.BillingVariantMonthly, b.BillingVariantYearly}
}

func (b Billing) Enabled() bool {
	for _, f := range b.fields() {
		if f == "" {
			return false
		}
	}
	return true
}

func (b Billing) validate() error {
	set := 0
	for _, f := range b.fields() {
		if f != "" {
			set++
		}
	}
	if set != 0 && set != len(b.fields()) {
		return errors.New("billing is half configured: set every LEMONSQUEEZY_ variable or none")
	}
	if b.billingProblem != nil {
		return b.billingProblem
	}
	if set != 0 && b.BillingTrialPeriod <= 0 {
		return errors.New("TRIAL_PERIOD must be longer than zero")
	}
	if set != 0 && b.BillingTestModeRaw != "true" && b.BillingTestModeRaw != "false" {
		return errors.New("LEMONSQUEEZY_TEST_MODE must be true or false once billing is set: a forgotten value must not open live payments")
	}
	return nil
}
