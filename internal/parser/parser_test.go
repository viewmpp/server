package parser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"server/internal/contract"
	"strings"
	"testing"
)

func TestParseEnforcesResponseLimit(t *testing.T) {
	tests := map[string]struct {
		size    int
		wantErr bool
	}{
		"at limit":   {size: contract.MaxBytes},
		"over limit": {size: contract.MaxBytes + 1, wantErr: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, strings.Repeat("x", test.size))
			}))
			defer server.Close()

			client := Client{URL: server.URL, HTTP: server.Client()}
			data, err := client.Parse(context.Background(), http.NoBody, 0)
			if test.wantErr && err == nil {
				t.Fatal("accepted oversized parser response")
			}
			if !test.wantErr && (err != nil || len(data) != test.size) {
				t.Fatalf("response at limit: bytes=%d error=%v", len(data), err)
			}
		})
	}
}
