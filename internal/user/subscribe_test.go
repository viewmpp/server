package user

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscribeOnlyReturnsToOurOwnPages(t *testing.T) {
	h := &Handler{baseURL: "https://viewmpp.com"}

	tests := []struct {
		name    string
		referer string
		want    string
	}{
		{"our own page", "https://viewmpp.com/pricing", "/pricing"},
		{"our own page with a query", "https://viewmpp.com/pricing?from=menu", "/pricing?from=menu"},
		{"our own root", "https://viewmpp.com", "/"},
		{"no referer at all", "", "/"},
		{"another site", "https://evil.example/take-the-money", "/"},
		{"a lookalike host", "https://viewmpp.com.evil.example/pricing", "/"},
		{"a scheme-relative address", "//evil.example/pricing", "/"},
		{"plain http version of us", "http://viewmpp.com/pricing", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/subscribe", nil)
			if tt.referer != "" {
				r.Header.Set("Referer", tt.referer)
			}

			if got := h.backTo(r); got != tt.want {
				t.Errorf("backTo = %q, want %q: the subscribe form must never send anyone off the site", got, tt.want)
			}
		})
	}
}
