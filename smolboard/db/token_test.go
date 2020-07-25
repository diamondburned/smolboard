package db

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/pkg/errors"
)

func TestToken(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")
	token := owner.AuthToken

	t.Run("UnlimitedUse", func(t *testing.T) {
		tx := testBeginTx(t, d, token)

		// Create an unlimited use token.
		k, err := tx.CreateToken(-1)
		if err != nil {
			t.Fatal("Failed to create token:", err)
		}

		// Consume the token 10 times. None of this should trigger a fatal.
		for i := 0; i < 10; i++ {
			if err := tx.useToken(k.Token); err != nil {
				t.Fatal("Failed to consume unlimited use token:", err)
			}
		}

		// Ensure that the token is still -1.
		tokens, err := tx.ListTokens()
		if err != nil {
			t.Fatal("Failed to get tokens:", err)
		}

		if len(tokens) != 1 {
			t.Fatal("Invalid tokens returned:", tokens)
		}

		if eq := deep.Equal(tokens[0], *k); eq != nil {
			t.Fatal("Returned unlimited-use token inequality:", eq)
		}
	})

	t.Run("OneTimer", func(t *testing.T) {
		tx := testBeginTx(t, d, token)

		// Create a one-timer token.
		k, err := tx.CreateToken(1)
		if err != nil {
			t.Fatal("Failed to create token:", err)
		}

		// Use once.
		if err := tx.useToken(k.Token); err != nil {
			t.Fatal("Failed to consume one-timer token once:", err)
		}

		// Try and use again.
		if err := tx.useToken(k.Token); !errors.Is(err, ErrUnknownToken) {
			t.Fatal("Unexpected error after token expiry")
		}
	})
}

func TestTokenListAdmin(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	var tokens = make([]Token, 0, 15)

	// Create 10 owner tokens.
	t.Run("OwnerCreate", func(t *testing.T) {
		tx := testBeginTx(t, d, owner.AuthToken)

		k, err := tx.CreateToken(-1)
		if err != nil {
			t.Fatal("Failed to create unlimited use token:", err)
		}
		tokens = append(tokens, *k)

		for i := 1; i < 10; i++ {
			k, err := tx.CreateToken(i)
			if err != nil {
				t.Fatal("Failed to create n-use token:", err)
			}
			tokens = append(tokens, *k)
		}
	})

	admin := testPermissionSet[PermissionAdministrator].begin(t, d, owner)

	tx := testBeginTx(t, d, admin.AuthToken)

	// Read before creating.
	list, err := tx.ListTokens()
	if err != nil {
		t.Fatal("Failed to list tokens:", err)
	}

	if len(list) > 0 {
		t.Fatal("Unexpected tokens before creating any for admin:", list)
	}

	// Create 5 admin tokens.
	t.Run("AdminCreate", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			k, err := tx.CreateToken(i)
			if err != nil {
				t.Fatal("Failed to create n-use token:", err)
			}
			tokens = append(tokens, *k)
		}
	})

	list, err = tx.ListTokens()
	if err != nil {
		t.Fatal("Failed to list tokens:", err)
	}

	// Admin tokens start after the 10 owner tokens, so we slice that away.
	if eq := deep.Equal(list, tokens[10:]); eq != nil {
		t.Fatal("Unexpected token list returned:", eq)
	}
}

func TestTokenCreateDeny(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	for perm, test := range testPermissionSet {
		t.Run(perm.String(), func(t *testing.T) {
			// Skip owner.
			if perm == PermissionOwner {
				return
			}

			s := test.begin(t, d, owner)
			tx := testBeginTx(t, d, s.AuthToken)

			t.Run("UnlimitedUse", func(t *testing.T) {
				_, err := tx.CreateToken(-1)
				if !errors.Is(err, ErrActionNotPermitted) {
					t.Fatal("Unexpected error creating token as normal user:", err)
				}
			})

			// Skip owner and admin.
			if perm == PermissionAdministrator {
				return
			}

			t.Run("OneTimer", func(t *testing.T) {
				_, err := tx.CreateToken(1)
				if !errors.Is(err, ErrActionNotPermitted) {
					t.Fatal("Unexpected error creating token as normal user:", err)
				}
			})
		})
	}
}

func TestTokenTrustedDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	admin := testPermissionSet[PermissionAdministrator].begin(t, d, owner)
	token := testOneTimeToken(t, d, admin.AuthToken)

	user := testPermissionSet[PermissionTrusted].begin(t, d, owner)
	tx := testBeginTx(t, d, user.AuthToken)

	if err := tx.DeleteToken(token); !errors.Is(err, ErrActionNotPermitted) {
		t.Fatal("Unexpected error while deleting higher-up's token:", err)
	}
}

func TestTokenAdminDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	test := testPermissionSet[PermissionAdministrator]
	s := test.begin(t, d, owner)

	t.Run("Self", func(t *testing.T) {
		tx := testBeginTx(t, d, s.AuthToken)

		k, err := tx.CreateToken(10)
		if err != nil {
			t.Fatal("Failed to create a token:", err)
		}

		if err := tx.DeleteToken(k.Token); err != nil {
			t.Fatal("Failed to delete token:", err)
		}
	})

	t.Run("Others", func(t *testing.T) {
		admin := newTestUser(t, d, owner.AuthToken, "ヒメゴト", PermissionAdministrator)
		token := testOneTimeToken(t, d, admin.AuthToken)

		tx := testBeginTx(t, d, s.AuthToken)

		if err := tx.DeleteToken(token); !errors.Is(err, ErrActionNotPermitted) {
			t.Fatal("Unexpected error deleting owner's token:", err)
		}
	})

	t.Run("Owner", func(t *testing.T) {
		token := testOneTimeToken(t, d, owner.AuthToken)

		tx := testBeginTx(t, d, s.AuthToken)

		if err := tx.DeleteToken(token); !errors.Is(err, ErrActionNotPermitted) {
			t.Fatal("Unexpected error deleting owner's token:", err)
		}
	})
}

func TestTokenOwnerDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	admin := testPermissionSet[PermissionAdministrator].begin(t, d, owner)
	token := testOneTimeToken(t, d, admin.AuthToken)

	tx := testBeginTx(t, d, owner.AuthToken)

	if err := tx.DeleteToken(token); err != nil {
		t.Fatal("Failed to delete admin's token:", err)
	}
}
