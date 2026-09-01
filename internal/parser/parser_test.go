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
