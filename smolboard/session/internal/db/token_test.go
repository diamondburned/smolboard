package db

import (
	"testing"

	"github.com/pkg/errors"
)

func TestToken(t *testing.T) {
	d := newTestDatabase(t)

	t.Run("UnlimitedUse", func(t *testing.T) {
		// Create an unlimited use token.
		k := createToken(t, d, -1)

		// Consume the token 10 times. None of this should trigger a fatal.
		for i := 0; i < 10; i++ {
			if err := d.UseToken(k.Token); err != nil {
				t.Fatal("Failed to consume unlimited use token:", err)
			}
		}
	})

	t.Run("OneTimer", func(t *testing.T) {
		// Create a one-timer token.
		k := createToken(t, d, 1)

		// Use once.
		if err := d.UseToken(k.Token); err != nil {
			t.Fatal("Failed to consume one-timer token once:", err)
		}

		// Try and use again.
		if err := d.UseToken(k.Token); !errors.Is(err, ErrUnknownToken) {
			t.Fatal("Unexpected error after token expiry")
		}
	})
}

func createToken(t *testing.T, d *Database, use int) *Token {
	t.Helper()

	k, err := d.CreateToken(use)
	if err != nil {
		t.Fatal("Failed to create token:", err)
	}

	return k
}
