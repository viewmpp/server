package config

import (
	"testing"
	"time"
)

func fullBilling() Billing {
	b := Billing{
		BillingEnvRaw:        PaddleSandbox,
		BillingAPIKey:        "pdl_sdbx_apikey_example",
		BillingClientToken:   "test_example",
		BillingWebhookSecret: "pdl_ntfset_example",
		BillingPriceMonthly:  "pri_monthly",
		BillingPriceYearly:   "pri_yearly",
		BillingTrialPeriod:   7 * 24 * time.Hour,
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
	partial.BillingClientToken = ""
	if partial.Enabled() {
		t.Error("billing without a client token reports itself enabled")
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

func TestAForgottenEnvironmentDoesNotOpenLivePayments(t *testing.T) {
	for _, raw := range []string{"", "Sandbox", "production", "test"} {
		cfg := validProd()
		cfg.Billing = fullBilling()
		cfg.BillingEnvRaw = raw
		cfg.fillBilling()

		if err := cfg.Validate(); err == nil {
			t.Errorf("PADDLE_ENV=%q was accepted", raw)
		}
	}
}

func TestKeysMustMatchTheEnvironment(t *testing.T) {
	live := validProd()
	live.Billing = fullBilling()
	live.BillingEnvRaw = PaddleLive
	live.fillBilling()
	if err := live.Validate(); err == nil {
		t.Error("sandbox keys were accepted for live payments")
	}

	live.BillingAPIKey = "pdl_live_apikey_example"
	live.BillingClientToken = "live_example"
	if err := live.Validate(); err != nil || live.BillingSandbox {
		t.Errorf("live keys for live payments were refused or read as sandbox: %v", err)
	}

	sandbox := validProd()
	sandbox.Billing = fullBilling()
	sandbox.BillingClientToken = "live_example"
	if err := sandbox.Validate(); err == nil {
		t.Error("a live client token was accepted in the sandbox")
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
			t.Errorf("PADDLE_TEST_USERS=%q was accepted", raw)
		}
	}
}
