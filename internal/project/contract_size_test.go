package project

import (
	"bytes"
	"server/internal/contract"
	"testing"
)

func TestDecompressEnforcesContractLimit(t *testing.T) {
	tests := map[string]struct {
		size    int
		wantErr bool
	}{
		"at limit":   {size: contract.MaxBytes},
		"over limit": {size: contract.MaxBytes + 1, wantErr: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			compressed, err := compress(bytes.Repeat([]byte{'x'}, test.size))
			if err != nil {
				t.Fatalf("compress: %v", err)
			}

			decoded, err := decompress(compressed)
			if test.wantErr && err == nil {
				t.Fatal("accepted oversized stored contract")
			}
			if !test.wantErr && (err != nil || len(decoded) != test.size) {
				t.Fatalf("stored contract at limit: bytes=%d error=%v", len(decoded), err)
			}
		})
	}
}
