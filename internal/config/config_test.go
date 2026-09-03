package config

import (
	"strings"
	"testing"
)

func TestValidateRefusesLocalhostInProduction(t *testing.T) {
	cases := []struct {
		name    string
		env     string
		baseURL string
		wantErr bool
	}{
		{"dev keeps localhost", "dev", "http://localhost:4000", false},
		{"stage keeps localhost", "stage", "http://localhost:4000", false},
		{"prod refuses localhost", "prod", "http://localhost:4000", true},
		{"prod refuses loopback", "prod", "http://127.0.0.1:4000", true},
		{"prod refuses empty", "prod", "", true},
		{"prod refuses plain http", "prod", "http://viewmpp.com", true},
		{"prod accepts https", "prod", "https://viewmpp.com", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			cfg.Env = tc.env
			cfg.Proxies = 1
			cfg.BaseURL = tc.baseURL
			cfg.SecretKey = strings.Repeat("k", MinSecretKeyLength)

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
	cfg.Env = "prod"
	cfg.Proxies = 1
	cfg.BaseURL = "https://viewmpp.com"

	if cfg.Validate() == nil {
		t.Error("prod started without SECRET_KEY: every restart would invalidate the forms people have open")
	}

	cfg.SecretKey = strings.Repeat("k", MinSecretKeyLength)

	if err := cfg.Validate(); err != nil {
		t.Errorf("a configured key was still rejected: %v", err)
	}
}

func TestAShortSecretKeyIsRefused(t *testing.T) {
	cfg := Config{}
	cfg.Env = "prod"
	cfg.Proxies = 1
	cfg.BaseURL = "https://viewmpp.com"
	cfg.SecretKey = "abc"

	if cfg.Validate() == nil {
		t.Error("prod accepted a three character key: a signature is only as strong as what signs it")
	}
}

func TestOnlyProductionInsistsOnAKey(t *testing.T) {
	cfg := Config{}
	cfg.Env = "dev"

	cfg.fillSecretKey()

	if cfg.SecretKey == "" {
		t.Error("dev was left without a key: csrf tokens could not be signed at all")
	}
}

func TestProductionNeedsAProxyCount(t *testing.T) {
	cfg := Config{}
	cfg.Env = "prod"
	cfg.BaseURL = "https://viewmpp.com"
	cfg.SecretKey = strings.Repeat("k", MinSecretKeyLength)

	if cfg.Validate() == nil {
		t.Error("prod accepted PROXIES 0: every visitor would resolve to the proxy and share one rate limit")
	}
}
