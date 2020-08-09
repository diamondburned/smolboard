package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/diamondburned/smolboard/smolboard"
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

	var owner *smolboard.Session
	var ownerToken string

	t.Run("VerifyPassword", func(t *testing.T) {
		var k *smolboard.Session
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			k, err = tx.Signin("ひめありかわ1", "goodpassword", "")
			return err
		})
		if err != nil {
			t.Fatal("Failed to sign in:", err)
		}
		owner = k
		ownerToken = k.AuthToken
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			_, err = tx.Signin("ひめありかわ1", "badpassword", "")
			return err
		})
		if !errors.Is(err, smolboard.ErrInvalidPassword) {
			t.Fatal("Unexpected error with invalid password:", err)
		}
	})

	t.Run("UnknownUser", func(t *testing.T) {
		err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
			_, err = tx.Signin("ヒメゴト", "mehpassword", "")
			return err
		})
		if !errors.Is(err, smolboard.ErrInvalidPassword) {
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
		if !errors.Is(err, smolboard.ErrUsernameTaken) {
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
			var k *smolboard.Session
			err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
				k, err = tx.Signin("かぐやありかわ", "12121212", "")
				return err
			})
			if err != nil {
				t.Fatal("Failed to sign in:", err)
			}
			token = k.AuthToken
		})

		// owner test
		t.Run("List", func(t *testing.T) {
			tx := testBeginTx(t, d, ownerToken)

			u, err := tx.Users(100, 0)
			if err != nil {
				t.Fatal("Failed to get users:", err)
			}

			if len(u) != 2 {
				t.Fatalf("Invalid users slice: %#v", u)
			}
		})

		t.Run("Promote", func(t *testing.T) {
			tx := testBeginTx(t, d, ownerToken)

			if err := tx.PromoteUser("かぐやありかわ", smolboard.PermissionTrusted); err != nil {
				t.Fatal("Failed to promote user to trusted:", err)
			}

			err := tx.PromoteUser("かぐやありかわ", smolboard.PermissionOwner)
			if !errors.Is(err, smolboard.ErrActionNotPermitted) {
				t.Fatal("Unexpected error while trying to promote user to owner:", err)
			}
		})

		t.Run("Get", func(t *testing.T) {
			tx := testBeginTx(t, d, token)

			var tests = []struct {
				name string
				perm smolboard.Permission
			}{
				{"ひめありかわ1", smolboard.PermissionOwner},
				{"かぐやありかわ", smolboard.PermissionTrusted},
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

		if _, err := tx.User("usernotfounderrpls"); !errors.Is(err, smolboard.ErrUserNotFound) {
			t.Fatal("Unexpected error getting non-existent user:", err)
		}

		if _, err := tx.User("invalid user"); !errors.Is(err, smolboard.ErrUserNotFound) {
			t.Fatal("Unexpected error getting illegal named user:", err)
		}
	})

	t.Run("ChangePassword", func(t *testing.T) {
		err := d.AcquireGuest(context.TODO(), func(tx *Transaction) error {
			_, err := tx.Signin(d.Config.Owner, "goodpassword", "")
			return err
		})
		if err != nil {
			t.Fatal("Failed to create a second session")
		}

		err = d.Acquire(context.TODO(), ownerToken, func(tx *Transaction) error {
			return tx.ChangePassword("otokonoko")
		})
		if err != nil {
			t.Fatal("Failed to change password:", err)
		}

		// Reacquire the sesssion with the same token. This should work.
		err = d.Acquire(context.TODO(), ownerToken, func(tx *Transaction) error {
			s, err := tx.Sessions()
			if err != nil {
				return errors.Wrap(err, "Failed to get sessions")
			}

			if len(s) != 1 {
				return fmt.Errorf("Unexpected sessions count: %d != 1", len(s))
			}

			if s[0].ID != owner.ID {
				return fmt.Errorf("Unexpected sessions returned: %#v", s)
			}

			return nil
		})
		if err != nil {
			t.Fatal("Failed to finish testing after password deletion:", err)
		}

		err = d.AcquireGuest(context.TODO(), func(tx *Transaction) error {
			// use the old passwrod
			_, err := tx.Signin(d.Config.Owner, "goodpassword", "")
			return err
		})
		if !errors.Is(err, smolboard.ErrInvalidPassword) {
			t.Fatal("Unexpected error using old password:", err)
		}

		err = d.AcquireGuest(context.TODO(), func(tx *Transaction) error {
			// use the new password
			_, err := tx.Signin(d.Config.Owner, "otokonoko", "")
			return err
		})
		if err != nil {
			t.Fatal("Unexpected error using new password:", err)
		}
	})

	t.Run("DeleteOwner", func(t *testing.T) {
		tx := testBeginTx(t, d, ownerToken)
		if err := tx.DeleteUser("ひめありかわ1"); !errors.Is(err, smolboard.ErrOwnerAccountStays) {
			t.Fatal("Unexpected error while deleting owner:", err)
		}
	})
}

func TestAdminDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	perm := smolboard.PermissionAdministrator
	test := testPermissionSet[perm]

	// Initialize the user session who will perform the delete.
	user := test.begin(t, d, owner)

	for _, lower := range test.passPerms {
		// Ignore owner.
		if lower == smolboard.PermissionOwner {
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
			if !errors.Is(err, smolboard.ErrActionNotPermitted) {
				t.Fatal("Unexpected error deleting equal-perm user:", err)
			}
		})

		// Delete the user anyway to free it up for the next turn.
		t.Run("TargetCleanup", func(t *testing.T) {
			tx := testBeginTx(t, d, owner.AuthToken)

			if err := tx.DeleteUser(target.Username); err != nil {
				if !errors.Is(err, smolboard.ErrUserNotFound) {
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

	err := tx.createUser("ひめ　ありかわ", "", smolboard.PermissionUser)
	if !errors.Is(err, smolboard.ErrIllegalName) {
		t.Fatal("Unexpected error while creating illegal-name user:", err)
	}
}

func TestShortPassword(t *testing.T) {
	tx := (*Transaction)(nil)

	err := tx.createUser("a", "b", smolboard.PermissionUser)
	if !errors.Is(err, smolboard.ErrPasswordTooShort) {
		t.Fatal("Unexpected error while creating a password too short:", err)
	}
}
