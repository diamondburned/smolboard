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
		var k *Session
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			k, err = tx.Signin("ひめありかわ1", "goodpassword", "")
			return err
		})
		if err != nil {
			t.Fatal("Failed to sign in:", err)
		}
		ownerToken = k.AuthToken
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			_, err = tx.Signin("ひめありかわ1", "badpassword", "")
			return err
		})
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatal("Unexpected error with invalid password:", err)
		}
	})

	t.Run("UnknownUser", func(t *testing.T) {
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			_, err = tx.Signin("ヒメゴト", "mehpassword", "")
			return err
		})
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatal("Unexpected error with invalid password:", err)
		}
	})

	// Run a subtest to test for duplicate user.
	t.Run("DuplicateOwner", func(t *testing.T) {
		k := testOneTimeToken(t, d, ownerToken)

		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			_, err = tx.Signup("ひめありかわ1", "12345678", k, "")
			return err
		})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatal("Unexpected error while creating duplicate user:", err)
		}
	})

	t.Run("User", func(t *testing.T) {
		k := testOneTimeToken(t, d, ownerToken)

		t.Run("Create", func(t *testing.T) {
			err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
				_, err = tx.Signup("かぐやありかわ", "12121212", k, "")
				return err
			})
			if err != nil {
				t.Fatal("Failed to sign up:", err)
			}
		})

		var token string

		t.Run("Signin", func(t *testing.T) {
			var k *Session
			err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
				k, err = tx.Signin("かぐやありかわ", "12121212", "")
				return err
			})
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

		t.Run("Get", func(t *testing.T) {
			tx := testBeginTx(t, d, token)

			var tests = []struct {
				name string
				perm Permission
			}{
				{"ひめありかわ1", PermissionOwner},
				{"かぐやありかわ", PermissionTrusted},
			}

			for _, test := range tests {
				u, err := tx.User(test.name)
				if err != nil {
					t.Fatal("Unexpected error getting user:", err)
				}

				if u.Username != test.name {
					t.Fatalf("Unexpected username with %q: %q", test.name, u.Username)
				}

				if u.Permission != test.perm {
					t.Fatal("Unexpected permission:", u.Permission)
				}
			}
		})

		t.Run("Delete", func(t *testing.T) {
			tx := testBeginTx(t, d, token)

			if err := tx.DeleteUser("かぐやありかわ"); err != nil {
				t.Fatal("Failed to delete user:", err)
			}
		})
	})

	t.Run("GetInvalid", func(t *testing.T) {
		tx := testBeginTx(t, d, ownerToken)

		if _, err := tx.User("usernotfounderrpls"); !errors.Is(err, ErrUserNotFound) {
			t.Fatal("Unexpected error getting non-existent user:", err)
		}

		if _, err := tx.User("invalid user"); !errors.Is(err, ErrUserNotFound) {
			t.Fatal("Unexpected error getting illegal named user:", err)
		}
	})

	t.Run("DeleteOwner", func(t *testing.T) {
		tx := testBeginTx(t, d, ownerToken)
		if err := tx.DeleteUser("ひめありかわ1"); !errors.Is(err, ErrOwnerAccountStays) {
			t.Fatal("Unexpected error while deleting owner:", err)
		}
	})
}

func TestAdminDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	perm := PermissionAdministrator
	test := testPermissionSet[perm]

	// Initialize the user session who will perform the delete.
	user := test.begin(t, d, owner)

	for _, lower := range test.passPerms {
		// Ignore owner.
		if lower == PermissionOwner {
			continue
		}

		// Initialize the user to be deleted.
		target := newTestUser(t, d, owner.AuthToken, "ヒメゴト", lower)

		t.Run(lower.String(), func(t *testing.T) {
			// Start a transaction with the user account.
			tx := testBeginTx(t, d, user.AuthToken)

			if lower != perm {
				if err := tx.DeleteUser(target.Username); err != nil {
					t.Fatalf("Failed to delete user with perm %v: %v", lower, err)
				}
				return
			}

			// User has same permission as target, so try and get the
			// right error.

			err := tx.DeleteUser(target.Username)
			if !errors.Is(err, ErrActionNotPermitted) {
				t.Fatal("Unexpected error deleting equal-perm user:", err)
			}
		})

		// Delete the user anyway to free it up for the next turn.
		t.Run("TargetCleanup", func(t *testing.T) {
			tx := testBeginTx(t, d, owner.AuthToken)

			if err := tx.DeleteUser(target.Username); err != nil {
				if !errors.Is(err, ErrUserNotFound) {
					t.Fatalf("Unexpected error during cleanup perm %v: %q", lower, err)
				}
			}
		})
	}

	t.Run("UserSelfCleanup", func(t *testing.T) {
		tx := testBeginTx(t, d, user.AuthToken)

		if err := tx.DeleteUser(user.Username); err != nil {
			t.Fatal("Unexpected error while cleaning up self:", err)
		}
	})
}

func TestIllegalUser(t *testing.T) {
	tx := (*Transaction)(nil)

	err := tx.createUser("ひめ　ありかわ", "", PermissionUser)
	if !errors.Is(err, ErrIllegalName) {
		t.Fatal("Unexpected error while creating illegal-name user:", err)
	}
}

func TestShortPassword(t *testing.T) {
	tx := (*Transaction)(nil)

	err := tx.createUser("a", "b", PermissionUser)
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatal("Unexpected error while creating a password too short:", err)
	}
}
