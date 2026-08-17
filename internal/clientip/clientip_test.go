package clientip

import (
	"net/http"
	"net/http/httptest"
	"server/internal/assert"
	"testing"
)

func resolve(t *testing.T, proxies int, remote string, forwarded ...string) string {
	t.Helper()

	res, err := NewResolver(proxies)
	assert.NilError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for _, value := range forwarded {
		r.Header.Add(header, value)
	}

	return res.Get(r)
}

func TestWithoutProxiesTheHeaderIsIgnored(t *testing.T) {
	got := resolve(t, 0, "203.0.113.5:1234", "1.2.3.4")

	if got != "203.0.113.5" {
		t.Fatalf("client ip = %q, want %q: a spoofed header must not decide identity when no proxy is trusted", got, "203.0.113.5")
	}
}

func TestOneProxyTakesTheRightmostEntry(t *testing.T) {
	got := resolve(t, 1, "10.0.0.1:5000", "1.2.3.4, 198.51.100.7")

	if got != "198.51.100.7" {
		t.Fatalf("client ip = %q, want %q", got, "198.51.100.7")
	}
}

func TestSpoofedEntriesToTheLeftAreDiscarded(t *testing.T) {
	got := resolve(t, 1, "10.0.0.1:5000", "attacker-supplied, 9.9.9.9, 198.51.100.7")

	if got != "198.51.100.7" {
		t.Fatalf("client ip = %q, want %q: only the last trusted hop counts", got, "198.51.100.7")
	}
}

func TestFallsBackToRemoteAddrWhenTheHeaderIsMissing(t *testing.T) {
	got := resolve(t, 1, "203.0.113.9:443")

	if got != "203.0.113.9" {
		t.Fatalf("client ip = %q, want %q: a missing header must not yield an empty key", got, "203.0.113.9")
	}
}

func TestNeverResolvesToAnEmptyKey(t *testing.T) {
	cases := []struct {
		name      string
		proxies   int
		remote    string
		forwarded []string
	}{
		{"no proxies, no header", 0, "203.0.113.5:1234", nil},
		{"no proxies, empty header", 0, "203.0.113.5:1234", []string{""}},
		{"one proxy, no header", 1, "203.0.113.5:1234", nil},
		{"one proxy, empty header", 1, "203.0.113.5:1234", []string{""}},
		{"two proxies, one entry", 2, "203.0.113.5:1234", []string{"198.51.100.7"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolve(t, tc.proxies, tc.remote, tc.forwarded...); got == "" {
				t.Fatal("client ip is empty: every request would share one rate-limit bucket")
			}
		})
	}
}

func TestContextRoundTrip(t *testing.T) {
	r := SetContext(httptest.NewRequest(http.MethodGet, "/", nil), "198.51.100.7")

	assert.Equal(t, From(r), "198.51.100.7")
}

func TestFromPanicsWithoutTheMiddleware(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("From did not panic on a request that never passed through the middleware")
		}
	}()

	From(httptest.NewRequest(http.MethodGet, "/", nil))
}
