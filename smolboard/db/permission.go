package db

import (
	"database/sql"

	"github.com/pkg/errors"
)

type Permission uint8

var ErrInvalidPermission = errors.New("invalid permission")

const (
	// PermissionNormal is a normal user's base permission. It allows uploading.
	PermissionNormal Permission = iota
	// PermissionTrusted has access to posts marked as trusted-only. Trusted
	// users can mark a post as trusted.
	PermissionTrusted
	// PermissionAdministrator can create limited use tokens as well as banning
	// and promoting people up to Trusted. This permission inherits all
	// permissions above.
	PermissionAdministrator
	// PermissionOwner indicates the owner of the image board. The owner can
	// create unlimited tokens and inherits all permissions above. They are also
	// the only person that can promote a person to Administrator.
	PermissionOwner // if username == Owner
)

func (p Permission) String() string {
	switch p {
	case PermissionNormal:
		return "Normal"
	case PermissionTrusted:
		return "Trusted"
	case PermissionAdministrator:
		return "Administrator"
	case PermissionOwner:
		return "Owner"
	default:
		return "???"
	}
}

// func (d *Tran)

// Permission scans for the permission of that user.
func (d *Transaction) permission(user string) (perm Permission, err error) {
	err = d.
		QueryRow("SELECT permission FROM users WHERE username = ?", user).
		Scan(&perm)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUserNotFound
		}

		return 0, errors.Wrap(err, "Failed to scan permission")
	}

	return perm, nil
}

// Permission scans for the permissions, or returns action not permitted if the
// user does not exist.
func (d *Transaction) Permission() (perm Permission, err error) {
	// Zero-value; allow guests.
	if d.Session.ID == 0 {
		return PermissionNormal, nil
	}

	return d.permission(d.Session.Username)
}

// HasPermission returns nil if the user has the given permission. If inclusive
// is true, then nil is returned if the user has the same permission as min.
func (d *Transaction) HasPermission(min Permission, inclusive bool) error {
	// Is this a valid permission?
	if min < 0 || min > PermissionOwner {
		return ErrInvalidPermission
	}

	p, err := d.Permission()
	if err != nil {
		return err
	}

	if p > min || (inclusive && p == min) {
		return nil
	}

	// Else, return forbidden.
	return ErrActionNotPermitted
}

func (d *Transaction) IsUserOrHasPermOver(min Permission, user string) error {
	if d.Session.Username == user {
		return nil
	}
	return d.HasPermOverUser(min, user)
}

// HasPermOverUser checks that the current user has at least the given minimum
// permission and has a higher permission than the target user.
func (d *Transaction) HasPermOverUser(min Permission, user string) error {
	// Is this a valid permission?
	if min < 0 || min > PermissionOwner {
		return ErrInvalidPermission
	}

	p, err := d.Permission()
	if err != nil {
		return err
	}

	// If the target permission is the same or larger than the current user's
	// permission and the user is different, then reject.
	if p < min {
		return ErrActionNotPermitted
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
		return ErrActionNotPermitted
	}

	// At this point:
	// 1. The current user has more or same permission than what's required.
	// 2. The target has a lower permission than the current user.
	// 3. The target user is not the current user.
	return nil
}
