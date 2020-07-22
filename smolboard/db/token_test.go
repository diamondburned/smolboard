package db

import (
	"testing"

	"github.com/pkg/errors"
)

func TestToken(t *testing.T) {
	d := newTestDatabase(t)

	token := testNewOwnerToken(t, d, "ひめありかわ", "goodpassword")
	tx := testBeginTx(t, d, token)

	t.Run("UnlimitedUse", func(t *testing.T) {
		// Create an unlimited use token.
		k, err := tx.CreateToken(-1)
		if err != nil {
			t.Fatal("Failed to create token:", err)
		}

		// Consume the token 10 times. None of this should trigger a fatal.
		for i := 0; i < 10; i++ {
			if err := tx.UseToken(k.Token); err != nil {
				t.Fatal("Failed to consume unlimited use token:", err)
			}
		}
	})

	t.Run("OneTimer", func(t *testing.T) {
		// Create a one-timer token.
		k, err := tx.CreateToken(1)
		if err != nil {
			t.Fatal("Failed to create token:", err)
		}

		// Use once.
		if err := tx.UseToken(k.Token); err != nil {
			t.Fatal("Failed to consume one-timer token once:", err)
		}

		// Try and use again.
		if err := tx.UseToken(k.Token); !errors.Is(err, ErrUnknownToken) {
			t.Fatal("Unexpected error after token expiry")
		}
	})
}
