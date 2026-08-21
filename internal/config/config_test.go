package config

import "testing"

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
		{"prod refuses plain http", "prod", "http://mppviewer.com", true},
		{"prod accepts https", "prod", "https://mppviewer.com", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			cfg.Env = tc.env
			cfg.BaseURL = tc.baseURL

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
