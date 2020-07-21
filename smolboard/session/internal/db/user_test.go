package db

import (
	"testing"

	"github.com/pkg/errors"
)

func TestUser(t *testing.T) {
	d := newTestDatabase(t)

	t.Run("CreateUser", func(t *testing.T) {
		_, err := d.NewUser("ひめありかわ1", "goodpassword", PermissionNormal)
		if err != nil {
			t.Fatal("Failed to make a new user:", err)
		}
	})

	t.Run("VerifyPassword", func(t *testing.T) {
		u, err := d.User("ひめありかわ1")
		if err != nil {
			t.Fatal("Failed to get user:", err)
		}

		if err := u.VerifyPassword("goodpassword"); err != nil {
			t.Fatal("Failed to verify password:", err)
		}
	})

	// Run a subtest to test for duplicate user.
	t.Run("DuplicateUser", func(t *testing.T) {
		_, err := d.NewUser("ひめありかわ1", "12345678", PermissionNormal)
		if !errors.Is(err, ErrIllegalName) {
			t.Fatal("Unexpected name while creating duplicate user:", err)
		}
	})

	t.Run("PromoteUser", func(t *testing.T) {
		if err := d.SetUserPermission("ありかわひめ1", PermissionTrusted); err != nil {
			t.Fatal("Failed to promote user to trusted:", err)
		}

		err := d.SetUserPermission("ありかわひめ1", PermissionOwner)
		if !errors.Is(err, ErrInvalidPermission) {
			t.Fatal("Unexpected error while trying to promote user to owner")
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		if err := d.DeleteUser("ひめありかわ1"); err != nil {
			t.Fatal("Failed to delete user:", err)
		}

		if _, err := d.User("ひめありかわ1"); !errors.Is(err, ErrUserNotFound) {
			t.Fatal("Unexpected error while getting after deleting:", err)
		}
	})
}

func TestIllegalUser(t *testing.T) {
	// nil db is fine
	var d *Database

	_, err := d.NewUser("ひめ　ありかわ", "", PermissionNormal)
	if !errors.Is(err, ErrIllegalName) {
		t.Fatal("Unexpected error while creating illegal-name user:", err)
	}
}

func TestShortPassword(t *testing.T) {
	var d *Database

	_, err := d.NewUser("a", "b", PermissionNormal)
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatal("Unexpected error while creating a password too short:", err)
	}
}
