package fixtures

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"server/internal/assert"
	"testing"
)

func TestFixturesMatchParser(t *testing.T) {
	cases := map[string][]byte{
		"mpp8.json":          mpp8,
		"mpp9.json":          mpp9,
		"mpp12.json":         mpp12,
		"mpp14baseline.json": mpp14Baseline,
		"cyrillic.json":      cyrillic,
		"mspdi.json":         mspdi,
	}

	for name, embedded := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "parser", "fixtures", name)

			original, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				t.Skipf("parser fixtures not available at %s", path)
			}
			assert.NilError(t, err)

			if sum(bytes.TrimSpace(original)) != sum(bytes.TrimSpace(embedded)) {
				t.Errorf("%s drifted from parser/fixtures/%s\n embedded: %s\n original: %s",
					name, name, sum(embedded), sum(original))
			}
		})
	}
}

func TestByDemoFallsBackToRealMpp(t *testing.T) {
	assert.Equal(t, ByDemo("").FileName, fallback.FileName)
	assert.Equal(t, ByDemo("nonsense").FileName, fallback.FileName)
	assert.Equal(t, ByDemo("cyrillic").FileName, "виадук.mpp")
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
