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

const (
	PaddleSandbox = "sandbox"
	PaddleLive    = "live"
)

type Billing struct {
	BillingEnvRaw        string
	BillingSandbox       bool
	BillingAPIKey        string
	BillingClientToken   string
	BillingWebhookSecret string
	BillingPriceMonthly  string
	BillingPriceYearly   string
	BillingTestUsersRaw  string
	BillingTestUsers     []int64
	BillingTrialPeriod   time.Duration

	billingProblem error
}

func (cfg *Config) loadBilling() {
	flag.StringVar(&cfg.BillingEnvRaw, "paddle-env", env.GetString("PADDLE_ENV", ""), "sandbox or live")
	flag.StringVar(&cfg.BillingAPIKey, "paddle-api-key", env.GetString("PADDLE_API_KEY", ""), "paddle api key")
	flag.StringVar(&cfg.BillingClientToken, "paddle-client-token", env.GetString("PADDLE_CLIENT_TOKEN", ""),
		"paddle client-side token for paddle.js")
	flag.StringVar(&cfg.BillingWebhookSecret, "paddle-webhook-secret", env.GetString("PADDLE_WEBHOOK_SECRET", ""),
		"secret paddle signs webhooks with")
	flag.StringVar(&cfg.BillingPriceMonthly, "paddle-price-monthly", env.GetString("PADDLE_PRICE_MONTHLY", ""),
		"paddle price id of the monthly plan")
	flag.StringVar(&cfg.BillingPriceYearly, "paddle-price-yearly", env.GetString("PADDLE_PRICE_YEARLY", ""),
		"paddle price id of the yearly plan")
	flag.StringVar(&cfg.BillingTestUsersRaw, "paddle-test-users", env.GetString("PADDLE_TEST_USERS", ""),
		"comma separated account ids that see payments while in the sandbox")
	flag.DurationVar(&cfg.BillingTrialPeriod, "trial-period", env.GetDuration("TRIAL_PERIOD", 7*24*time.Hour),
		"length of the free pro trial once payments are offered")
}

func (b *Billing) fillBilling() {
	b.BillingSandbox = b.BillingEnvRaw == PaddleSandbox
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
			return nil, fmt.Errorf("PADDLE_TEST_USERS holds %q, which is not an account id", part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (b Billing) fields() []string {
	return []string{b.BillingAPIKey, b.BillingClientToken, b.BillingWebhookSecret, b.BillingPriceMonthly, b.BillingPriceYearly}
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
		return errors.New("billing is half configured: set every PADDLE_ variable or none")
	}
	if b.billingProblem != nil {
		return b.billingProblem
	}
	if set == 0 {
		return nil
	}
	if b.BillingTrialPeriod <= 0 {
		return errors.New("TRIAL_PERIOD must be longer than zero")
	}
	if b.BillingEnvRaw != PaddleSandbox && b.BillingEnvRaw != PaddleLive {
		return errors.New("PADDLE_ENV must be sandbox or live once billing is set: a forgotten value must not open live payments")
	}

	wantKey, wantToken := "pdl_live_", "live_"
	if b.BillingSandbox {
		wantKey, wantToken = "pdl_sdbx_", "test_"
	}
	if !strings.HasPrefix(b.BillingAPIKey, wantKey) {
		return fmt.Errorf("PADDLE_API_KEY is not a %s key: keys only work against their own environment", b.BillingEnvRaw)
	}
	if !strings.HasPrefix(b.BillingClientToken, wantToken) {
		return fmt.Errorf("PADDLE_CLIENT_TOKEN is not a %s token", b.BillingEnvRaw)
	}
	return nil
}
