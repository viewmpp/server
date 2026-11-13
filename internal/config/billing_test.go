package config

import (
	"testing"
	"time"
)

func fullBilling() Billing {
	b := Billing{
		BillingAPIKey:         "key",
		BillingWebhookSecret:  "secret",
		BillingStoreID:        "1",
		BillingVariantMonthly: "2370696",
		BillingVariantYearly:  "2370717",
		BillingTestModeRaw:    "true",
		BillingTrialPeriod:    7 * 24 * time.Hour,
	}
	b.fillBilling()
	return b
}

func TestBillingIsOffUntilEveryVariableIsSet(t *testing.T) {
	if (Billing{}).Enabled() {
		t.Error("billing with nothing set reports itself enabled")
	}
	if !fullBilling().Enabled() {
		t.Error("billing with everything set reports itself disabled")
	}

	partial := fullBilling()
	partial.BillingWebhookSecret = ""
	if partial.Enabled() {
		t.Error("billing without a webhook secret reports itself enabled")
	}
}

func TestAServerWithHalfABillingSetupRefusesToStart(t *testing.T) {
	for _, env := range []string{"prod", "dev"} {
		t.Run(env, func(t *testing.T) {
			cfg := validProd()
			cfg.AppEnv = env
			cfg.Billing = fullBilling()
			cfg.BillingAPIKey = ""

			if err := cfg.Validate(); err == nil {
				t.Error("a half configured billing was accepted; payments would be taken and never applied")
			}
		})
	}
}

func TestProductionStartsWithBillingOffOrFullyOn(t *testing.T) {
	off := validProd()
	if err := off.Validate(); err != nil {
		t.Errorf("production without billing was refused: %v", err)
	}

	on := validProd()
	on.Billing = fullBilling()
	if err := on.Validate(); err != nil {
		t.Errorf("production with full billing was refused: %v", err)
	}
}

func TestAForgottenTestModeDoesNotOpenLivePayments(t *testing.T) {
	for _, raw := range []string{"", "yes", "TRUE", "1"} {
		cfg := validProd()
		cfg.Billing = fullBilling()
		cfg.BillingTestModeRaw = raw
		cfg.fillBilling()

		if err := cfg.Validate(); err == nil {
			t.Errorf("LEMONSQUEEZY_TEST_MODE=%q was accepted", raw)
		}
	}

	live := validProd()
	live.Billing = fullBilling()
	live.BillingTestModeRaw = "false"
	live.fillBilling()
	if err := live.Validate(); err != nil || live.BillingTestMode {
		t.Errorf("an explicit false was refused or read as test mode: %v", err)
	}
}

func TestTestUsersAreReadAsAccountIDs(t *testing.T) {
	b := fullBilling()
	b.BillingTestUsersRaw = " 12, 7 ,,"
	b.fillBilling()
	if b.validate() != nil || len(b.BillingTestUsers) != 2 || b.BillingTestUsers[0] != 12 || b.BillingTestUsers[1] != 7 {
		t.Errorf("test users = %v, err = %v", b.BillingTestUsers, b.validate())
	}

	for _, raw := range []string{"me@example.com", "12,abc", "-3", "0"} {
		b.BillingTestUsersRaw = raw
		b.fillBilling()
		if b.validate() == nil {
			t.Errorf("LEMONSQUEEZY_TEST_USERS=%q was accepted", raw)
		}
	}
}
