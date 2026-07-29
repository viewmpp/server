package fixture

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestFixturesMatchParser(t *testing.T) {
	cases := map[string][]byte{
		"mpp14baseline.json": mpp14Baseline,
		"cyrillic.json":      cyrillic,
	}

	for name, embedded := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "parser", "fixtures", name)

			original, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				t.Skipf("parser fixtures not available at %s", path)
			}
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			if sum(original) != sum(embedded) {
				t.Errorf("%s drifted from parser/fixtures/%s\n embedded: %s\n original: %s",
					name, name, sum(embedded), sum(original))
			}
		})
	}
}

func TestByDemoFallsBackToRealMpp(t *testing.T) {
	if got := ByDemo("").FileName; got != fallback.FileName {
		t.Errorf("empty demo: got %q, want %q", got, fallback.FileName)
	}
	if got := ByDemo("nonsense").FileName; got != fallback.FileName {
		t.Errorf("unknown demo: got %q, want %q", got, fallback.FileName)
	}
	if got := ByDemo("cyrillic").FileName; got != "виадук.mpp" {
		t.Errorf("cyrillic demo: got %q", got)
	}
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
