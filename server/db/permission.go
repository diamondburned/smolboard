package db

import (
	"database/sql"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

// Permission scans for the permission of that user. It returns -1 if there is
// an error.
func (d *Transaction) permission(user string) (perm smolboard.Permission, err error) {
	if user == d.config.Owner {
		return smolboard.PermissionOwner, nil
	}

	err = d.
		QueryRow("SELECT permission FROM users WHERE username = ?", user).
		Scan(&perm)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, smolboard.ErrUserNotFound
		}

		return -1, errors.Wrap(err, "Failed to scan permission")
	}

	return perm, nil
}

// Permission scans for the permissions, or returns action not permitted if the
// user does not exist.
func (d *Transaction) Permission() (perm smolboard.Permission, err error) {
	// Zero-value; allow guests.
	if d.Session.ID == 0 {
		return smolboard.PermissionGuest, nil
	}

	return d.permission(d.Session.Username)
}

// HasPermission returns nil if the user has the given permission. If inclusive
// is true, then nil is returned if the user has the same permission as min.
func (d *Transaction) HasPermission(min smolboard.Permission, inclusive bool) error {
	p, err := d.Permission()
	if err != nil {
		return err
	}

	return p.HasPermission(min, inclusive)
}

func (d *Transaction) IsUserOrHasPermOver(min smolboard.Permission, user string) error {
	if d.Session.Username == user {
		return nil
	}
	return d.HasPermOverUser(min, user)
}

// HasPermOverUser checks that the current user has at least the given minimum
// permission and has a higher permission than the target user.
func (d *Transaction) HasPermOverUser(min smolboard.Permission, user string) error {
	p, err := d.Permission()
	if err != nil {
		return err
	}

	// Accept a -1 permission.
	t, _ := d.permission(user)

	return p.HasPermOverUser(min, t, d.Session.Username, user)
}
