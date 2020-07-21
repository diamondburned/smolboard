package session

import (
	"testing"

	"github.com/pkg/errors"
)

func TestSessionActorOwner(t *testing.T) {
	d := newTestDatabase(t)
	d.cfg.Owner = "ひめありかわ1"
	d.cfg.MaxTokenUses = 2

	_, err := d.CreateOwner("password")
	if err != nil {
		t.Fatal("Failed to create owner:", err)
	}

	a, err := d.Signin(d.cfg.Owner, "password", "Android")
	if err != nil {
		t.Fatal("Failed to sign in:", err)
	}

	var token *Token

	t.Run("Token", func(t *testing.T) {
		// This should not work.
		_, err := a.CreateToken(5)
		if !errors.Is(err, ErrOverUseLimit) {
			t.Fatal("Unexpected error creating a big use count token:", err)
		}

		// This should work.
		_, err = a.CreateToken(1)
		if err != nil {
			t.Fatal("Unexpected error creating a one-timer token:", err)
		}

		token, err = a.CreateToken(-1)
		if err != nil {
			t.Fatal("Unexpected error creating an unlimited-use token:", err)
		}
	})

	t.Run("SessionNormal", func(t *testing.T) {
		testSessionActorNormal(t, token)
	})
}

func testSessionActorNormal(t *testing.T, token *Token) {

}

func testSessionActorTrusted(t *testing.T, token *Token) {}

func testSessionActorAdmin(t *testing.T, token *Token) {

}
