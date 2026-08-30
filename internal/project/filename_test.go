package project

import (
	"server/internal/assert"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "plan.mpp", "plan.mpp"},
		{"strips the path", "/tmp/deep/plan.mpp", "plan.mpp"},
		{"empty", "", ""},
		{"root", "/", ""},
		{"control characters", "pl\x00an\x1f.mpp", "plan.mpp"},
		{"cyrillic survives", "виадук.mpp", "виадук.mpp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, sanitizeFileName(tt.in), tt.want)
		})
	}
}

func TestSanitizeFileNameTruncatesOnRuneBoundary(t *testing.T) {
	widths := map[string]string{
		"two byte runes":   "виадук",
		"three byte runes": "план—",
		"four byte runes":  "🏗",
	}

	for name, unit := range widths {
		t.Run(name, func(t *testing.T) {
			got := sanitizeFileName(strings.Repeat(unit, 200) + ".mpp")

			if len(got) > maxFileNameBytes {
				t.Errorf("kept %d bytes, limit is %d", len(got), maxFileNameBytes)
			}
			if !utf8.ValidString(got) {
				t.Error("truncation produced invalid utf-8")
			}
		})
	}
}
