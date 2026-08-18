package project

import (
	"server/internal/assert"
	"testing"
)

func TestOpensNewShare(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{"private to public", AccessPrivate, AccessPublic, true},
		{"private to protected", AccessPrivate, AccessProtected, true},
		{"private to private", AccessPrivate, AccessPrivate, false},

		{"public to protected", AccessPublic, AccessProtected, false},
		{"protected to public", AccessProtected, AccessPublic, false},
		{"public to public", AccessPublic, AccessPublic, false},
		{"protected to protected", AccessProtected, AccessProtected, false},

		{"public to private", AccessPublic, AccessPrivate, false},
		{"protected to private", AccessProtected, AccessPrivate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, opensNewShare(tt.current, tt.next), tt.want)
		})
	}
}
