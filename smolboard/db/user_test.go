package db

import (
	"context"
	"testing"

	"github.com/pkg/errors"
)

func TestUser(t *testing.T) {
	d := newTestDatabase(t)
	d.Config.Owner = "ひめありかわ1"

	t.Run("CreateOwner", func(t *testing.T) {
		if err := d.createOwner("goodpassword"); err != nil {
			t.Fatal("Failed to make owner user:", err)
		}
	})

	var ownerToken string

	t.Run("VerifyPassword", func(t *testing.T) {
		k, err := d.Signin(context.Background(), "ひめありかわ1", "goodpassword", "")
		if err != nil {
			t.Fatal("Failed to sign in:", err)
		}
		ownerToken = k.AuthToken
	})

	// Run a subtest to test for duplicate user.
	t.Run("DuplicateOwner", func(t *testing.T) {
		k := testOneTimeToken(t, d, ownerToken)

		_, err := d.Signup(context.Background(), "ひめありかわ1", "12345678", k, "")
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatal("Unexpected error while creating duplicate user:", err)
		}
	})

	t.Run("User", func(t *testing.T) {
		k := testOneTimeToken(t, d, ownerToken)

		t.Run("Create", func(t *testing.T) {
			_, err := d.Signup(context.Background(), "かぐやありかわ", "12121212", k, "")
			if err != nil {
				t.Fatal("Failed to sign up:", err)
			}
		})

		var token string

		t.Run("Signin", func(t *testing.T) {
			k, err := d.Signin(context.Background(), "かぐやありかわ", "12121212", "")
			if err != nil {
				t.Fatal("Failed to sign in:", err)
			}
			token = k.AuthToken
		})

		t.Run("Promote", func(t *testing.T) {
			tx := testBeginTx(t, d, ownerToken)

			if err := tx.PromoteUser("かぐやありかわ", PermissionTrusted); err != nil {
				t.Fatal("Failed to promote user to trusted:", err)
			}

			err := tx.PromoteUser("かぐやありかわ", PermissionOwner)
			if !errors.Is(err, ErrActionNotPermitted) {
				t.Fatal("Unexpected error while trying to promote user to owner:", err)
			}
		})

		t.Run("Delete", func(t *testing.T) {
			tx := testBeginTx(t, d, token)

			if err := tx.DeleteUser("かぐやありかわ"); err != nil {
				t.Fatal("Failed to delete user:", err)
			}
		})
	})

	t.Run("DeleteOwner", func(t *testing.T) {
		tx := testBeginTx(t, d, ownerToken)
		if err := tx.DeleteUser("ひめありかわ1"); !errors.Is(err, ErrOwnerAccountStays) {
			t.Fatal("Unexpected error while deleting owner:", err)
		}
	})
}

func TestIllegalUser(t *testing.T) {
	_, err := NewUser("ひめ　ありかわ", "", PermissionNormal)
	if !errors.Is(err, ErrIllegalName) {
		t.Fatal("Unexpected error while creating illegal-name user:", err)
	}
}

func TestShortPassword(t *testing.T) {
	_, err := NewUser("a", "b", PermissionNormal)
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatal("Unexpected error while creating a password too short:", err)
	}
}
