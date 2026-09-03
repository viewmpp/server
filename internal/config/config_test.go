package config

import (
	"strings"
	"testing"
)

func TestValidateRefusesLocalhostInProduction(t *testing.T) {
	cases := []struct {
		name     string
		env      string
		baseURL  string
		diagAddr string
		wantErr  bool
	}{
		{"dev keeps localhost", "dev", "http://localhost:4000", "127.0.0.1:6060", false},
		{"stage keeps localhost", "stage", "http://localhost:4000", "127.0.0.1:6060", false},
		{"prod refuses localhost", "prod", "http://localhost:4000", "127.0.0.1:6060", true},
		{"prod refuses loopback", "prod", "http://127.0.0.1:4000", "127.0.0.1:6060", true},
		{"prod refuses empty base url", "prod", "", "127.0.0.1:6060", true},
		{"prod refuses plain http", "prod", "http://viewmpp.com", "127.0.0.1:6060", true},
		{"prod accepts https with diagnostics address ipv4", "prod", "https://viewmpp.com", "127.0.0.1:6060", false},
		{"prod accepts https with diagnostics address localhost", "prod", "https://viewmpp.com", "localhost:6060", false},
		{"prod refuses empty diagnostics address", "prod", "https://viewmpp.com", "", true},
		{"prod refuses diagnostics illegal port", "prod", "https://viewmpp.com", "127.0.0.1:4000", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			cfg.AppEnv = tc.env
			cfg.AppProxies = 1
			cfg.AppBaseURL = tc.baseURL
			cfg.SessionSecretKey = strings.Repeat("k", MinSecretKeyLength)
			cfg.DiagAddr = tc.diagAddr

			err := cfg.Validate()

			if tc.wantErr && err == nil {
				t.Fatalf("Validate() accepted BASE_URL %q in %s: a launch would publish a sitemap of unreachable URLs", tc.baseURL, tc.env)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() rejected a valid configuration: %v", err)
			}
		})
	}
}

func TestProductionNeedsASecretKey(t *testing.T) {
	cfg := Config{}
	cfg.AppEnv = "prod"
	cfg.AppProxies = 1
	cfg.AppBaseURL = "https://viewmpp.com"
	cfg.DiagAddr = "127.0.0.1:6060"

	if cfg.Validate() == nil {
		t.Error("prod started without SECRET_KEY: every restart would invalidate the forms people have open")
	}

	cfg.SessionSecretKey = strings.Repeat("k", MinSecretKeyLength)

	if err := cfg.Validate(); err != nil {
		t.Errorf("a configured key was still rejected: %v", err)
	}
}

func TestAShortSecretKeyIsRefused(t *testing.T) {
	cfg := Config{}
	cfg.AppEnv = "prod"
	cfg.AppProxies = 1
	cfg.AppBaseURL = "https://viewmpp.com"
	cfg.SessionSecretKey = "abc"

	if cfg.Validate() == nil {
		t.Error("prod accepted a three character key: a signature is only as strong as what signs it")
	}
}

func TestOnlyProductionInsistsOnAKey(t *testing.T) {
	cfg := Config{}
	cfg.AppEnv = "dev"

	cfg.fillSecretKey()

	if cfg.SessionSecretKey == "" {
		t.Error("dev was left without a key: csrf tokens could not be signed at all")
	}
}

func TestProductionNeedsAProxyCount(t *testing.T) {
	cfg := Config{}
	cfg.AppEnv = "prod"
	cfg.AppBaseURL = "https://viewmpp.com"
	cfg.SessionSecretKey = strings.Repeat("k", MinSecretKeyLength)

	if cfg.Validate() == nil {
		t.Error("prod accepted PROXIES 0: every visitor would resolve to the proxy and share one rate limit")
	}
}
