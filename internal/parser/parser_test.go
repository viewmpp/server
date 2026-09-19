package parser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"server/internal/contract"
	"strings"
	"testing"
	"time"
)

func TestParseEnforcesResponseLimit(t *testing.T) {
	tests := map[string]struct {
		status         int
		size           int
		wantOverflow   bool
		wantParseError bool
	}{
		"success at limit":   {status: http.StatusOK, size: contract.MaxBytes},
		"success over limit": {status: http.StatusOK, size: contract.MaxBytes + 1, wantOverflow: true},
		"error at limit":     {status: http.StatusBadRequest, size: contract.MaxBytes, wantParseError: true},
		"error over limit":   {status: http.StatusBadRequest, size: contract.MaxBytes + 1, wantOverflow: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, strings.Repeat("x", test.size))
			}))
			defer server.Close()

			client := Client{URL: server.URL, HTTP: server.Client()}
			data, err := client.Parse(context.Background(), http.NoBody, 0)
			if test.wantOverflow {
				if err == nil || !strings.Contains(err.Error(), "parser response exceeds") {
					t.Fatalf("overflow error=%v", err)
				}
				return
			}
			if test.wantParseError {
				var parseError *ParseError
				if !errors.As(err, &parseError) || parseError.Status != test.status {
					t.Fatalf("parse error=%v", err)
				}
				return
			}
			if err != nil || len(data) != test.size {
				t.Fatalf("response at limit: bytes=%d error=%v", len(data), err)
			}
		})
	}
}

func stubParser(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &Client{URL: server.URL, HTTP: server.Client()}
}

func TestHealthcheckAcceptsAHealthyParser(t *testing.T) {
	var path string

	client := stubParser(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"OK","env":"prod","version":"abc123"}`))
	})

	if err := client.Healthcheck(context.Background()); err != nil {
		t.Fatalf("a parser answering correctly was reported as broken: %v", err)
	}

	if path != healthcheckURL {
		t.Errorf("the probe asked for %q, want %q: PARSER_URL carries the host only", path, healthcheckURL)
	}
}

func TestParsePostsToTheParseEndpoint(t *testing.T) {
	var path, method string

	client := stubParser(t, func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		_, _ = w.Write([]byte(`{}`))
	})

	if _, err := client.Parse(context.Background(), strings.NewReader("x"), 1); err != nil {
		t.Fatalf("parse: %v", err)
	}

	if path != parseURL || method != http.MethodPost {
		t.Errorf("parse sent %s %q, want POST %q: the base url and the endpoint must not drift apart", method, path, parseURL)
	}
}

func TestHealthcheckRejectsAParserThatIsNotWell(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
	}{
		"service unavailable": {status: http.StatusServiceUnavailable, body: `{"status":"OK"}`},
		"another vocabulary":  {status: http.StatusOK, body: `{"status":"UP"}`},
		"empty status":        {status: http.StatusOK, body: `{"env":"prod"}`},
		"not json at all":     {status: http.StatusOK, body: `<html>gateway</html>`},
		"empty body":          {status: http.StatusOK, body: ``},
		"oversized body":      {status: http.StatusOK, body: strings.Repeat("x", maxHealthcheckBytes+1)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := stubParser(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			})

			if err := client.Healthcheck(context.Background()); err == nil {
				t.Error("the probe called this parser healthy")
			}
		})
	}
}

func TestHealthcheckFailsWhenTheParserIsUnreachable(t *testing.T) {
	client := &Client{URL: "http://127.0.0.1:1", HTTP: &http.Client{Timeout: time.Second}}

	if err := client.Healthcheck(context.Background()); err == nil {
		t.Error("nothing is listening there, and the probe was content")
	}
}

func TestEndpointsSurviveATrailingSlash(t *testing.T) {
	var path string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{URL: server.URL + "/", HTTP: server.Client()}

	if err := client.Healthcheck(context.Background()); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}

	if path != healthcheckURL {
		t.Errorf("a base url ending in a slash produced %q: one stray character must not move the endpoint", path)
	}
}
