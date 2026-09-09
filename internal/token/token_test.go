package token

import (
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

func TestTheVerificationCodeIsShortEnoughToTypeByHand(t *testing.T) {
	code, err := NewVerification(1, time.Hour)
	if err != nil {
		t.Fatalf("verification token: %v", err)
	}

	if got := len(code.Plaintext); got != 8 {
		t.Errorf("code is %d characters (%q), want 8: it is read off an email and typed into a form, so length is a cost paid by every new account", got, code.Plaintext)
	}

	if strings.ToUpper(code.Plaintext) != code.Plaintext {
		t.Errorf("code %q is not upper case, and the form normalises input to upper case before looking it up", code.Plaintext)
	}
}

func TestTheResetTokenStaysLong(t *testing.T) {
	reset, err := NewReset(1, time.Hour)
	if err != nil {
		t.Fatalf("reset token: %v", err)
	}

	if got := len(reset.Plaintext); got < 32 {
		t.Errorf("reset token is %d characters, want at least 32: it travels in a link and nobody types it, so there is no reason to trade entropy away", got)
	}
}

func TestTokensAreStoredAsAHashOfWhateverWasIssued(t *testing.T) {
	for name, make := range map[string]func() (*Token, error){
		"verification": func() (*Token, error) { return NewVerification(1, time.Hour) },
		"reset":        func() (*Token, error) { return NewReset(1, time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			issued, err := make()
			if err != nil {
				t.Fatalf("token: %v", err)
			}

			want := sha256.Sum256([]byte(issued.Plaintext))

			if string(issued.Hash) != string(want[:]) {
				t.Error("the stored hash is not the hash of the issued value: lookup is by hash, so a mismatch makes every code invalid")
			}

			if len(issued.Hash) != sha256.Size {
				t.Errorf("hash is %d bytes, want %d regardless of how long the code itself is", len(issued.Hash), sha256.Size)
			}
		})
	}
}

func TestTwoCodesAreNotTheSame(t *testing.T) {
	seen := make(map[string]bool, 64)

	for i := 0; i < 64; i++ {
		code, err := NewVerification(1, time.Hour)
		if err != nil {
			t.Fatalf("verification token: %v", err)
		}

		if seen[code.Plaintext] {
			t.Fatalf("%q was issued twice in 64 draws", code.Plaintext)
		}

		seen[code.Plaintext] = true
	}
}
