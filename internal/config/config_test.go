package config

import (
	"strings"
	"testing"
)

func validProd() Config {
	var cfg Config

	cfg.AppEnv = "prod"
	cfg.AppProxies = 1
	cfg.AppBaseURL = "https://viewmpp.com"
	cfg.SessionSecretKey = strings.Repeat("k", MinSecretKeyLength)
	cfg.DiagAddr = "127.0.0.1:6060"

	return cfg
}

func TestAGoodProductionConfigStarts(t *testing.T) {
	cfg := validProd()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("a valid production configuration was rejected: %v", err)
	}
}

func TestOutsideProductionNothingIsChecked(t *testing.T) {
	for _, env := range []string{"dev", "stage"} {
		t.Run(env, func(t *testing.T) {
			var cfg Config
			cfg.AppEnv = env
			cfg.AppBaseURL = "http://localhost:4000"
			cfg.DiagAddr = ":6060"

			if err := cfg.Validate(); err != nil {
				t.Fatalf("%s was held to production rules: %v", env, err)
			}
		})
	}
}

func TestProductionRefusesAnUnreachableBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
	}{
		{"empty", ""},
		{"localhost", "http://localhost:4000"},
		{"loopback", "http://127.0.0.1:4000"},
		{"plain http", "http://viewmpp.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validProd()
			cfg.AppBaseURL = tc.baseURL

			if cfg.Validate() == nil {
				t.Fatalf("prod accepted BASE_URL %q: it signs every reset link and the whole sitemap", tc.baseURL)
			}
		})
	}
}

func TestProductionKeepsDiagnosticsOnLoopback(t *testing.T) {
	cases := []struct {
		name     string
		diagAddr string
		wantErr  bool
	}{
		{"loopback address", "127.0.0.1:6060", false},
		{"localhost name", "localhost:6060", false},
		{"every interface", ":6060", true},
		{"public interface", "0.0.0.0:6060", true},
		{"empty", "", true},
		{"the application port", "127.0.0.1:4000", true},
		{"the database port", "127.0.0.1:5432", true},
		{"the parser port", "127.0.0.1:8080", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validProd()
			cfg.DiagAddr = tc.diagAddr

			err := cfg.Validate()

			if tc.wantErr && err == nil {
				t.Fatalf("prod accepted diagnostics on %q: expvar would answer to whoever can reach it", tc.diagAddr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("prod rejected diagnostics on %q: %v", tc.diagAddr, err)
			}
		})
	}
}

func TestProductionNeedsASecretKey(t *testing.T) {
	cfg := validProd()
	cfg.SessionSecretKey = ""

	if cfg.Validate() == nil {
		t.Error("prod started without SECRET_KEY: every restart would invalidate the forms people have open")
	}
}

func TestAShortSecretKeyIsRefused(t *testing.T) {
	cfg := validProd()
	cfg.SessionSecretKey = "abc"

	if cfg.Validate() == nil {
		t.Error("prod accepted a three character key: a signature is only as strong as what signs it")
	}
}

func TestProductionNeedsAProxyCount(t *testing.T) {
	cfg := validProd()
	cfg.AppProxies = 0

	if cfg.Validate() == nil {
		t.Error("prod accepted PROXIES 0: every visitor would resolve to the proxy and share one rate limit")
	}
}

func TestOnlyProductionInsistsOnAKey(t *testing.T) {
	var cfg Config
	cfg.AppEnv = "dev"

	cfg.fillSecretKey()

	if cfg.SessionSecretKey == "" {
		t.Error("dev was left without a key: csrf tokens could not be signed at all")
	}
}
