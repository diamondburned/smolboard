package db

import (
	"database/sql"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

// func (d *Tran)

// Permission scans for the permission of that user.
func (d *Transaction) permission(user string) (perm smolboard.Permission, err error) {
	err = d.
		QueryRow("SELECT permission FROM users WHERE username = ?", user).
		Scan(&perm)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, smolboard.ErrUserNotFound
		}

		return 0, errors.Wrap(err, "Failed to scan permission")
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
	// Is this a valid permission?
	if min < smolboard.PermissionGuest || min > smolboard.PermissionOwner {
		return smolboard.ErrInvalidPermission
	}

	p, err := d.Permission()
	if err != nil {
		return err
	}

	if p > min || (inclusive && p == min) {
		return nil
	}

	// Else, return forbidden.
	return smolboard.ErrActionNotPermitted
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
	// Is this a valid permission?
	if min < smolboard.PermissionGuest || min > smolboard.PermissionOwner {
		return smolboard.ErrInvalidPermission
	}

	p, err := d.Permission()
	if err != nil {
		return err
	}

	// If the target permission is the same or larger than the current user's
	// permission and the user is different, then reject.
	if p < min {
		return smolboard.ErrActionNotPermitted
	}

	// If the target user is the current user and the target permission is the
	// same or lower than the target, then allow.
	if d.Session.Username == user && p >= min {
		return nil
	}

	// At this point, p >= min. This means the user does indeed have more than
	// the required requirements. We now need to check that the target
	// permission has a lower permission.

	t, err := d.permission(user)
	if err != nil {
		return err
	}

	// If the target user has the same or higher permission, then deny.
	if t >= p {
		return smolboard.ErrActionNotPermitted
	}

	// At this point:
	// 1. The current user has more or same permission than what's required.
	// 2. The target has a lower permission than the current user.
	// 3. The target user is not the current user.
	return nil
}
