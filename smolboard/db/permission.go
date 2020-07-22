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

func (d *Transaction) Permission() (perm Permission, err error) {
	err = d.
		QueryRow("SELECT permission FROM users WHERE username = ?", d.session.Username).
		Scan(&perm)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrActionNotPermitted
		}

		return 0, errors.Wrap(err, "Failed to scan permission")
	}

	return perm, nil
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

// HasPermOrIsUser returns no error if the given user is the current user or the
// current user has more than the permission given.
func (d *Transaction) HasPermOrIsUser(min Permission, user string, inclusive bool) error {
	if user == d.session.Username {
		return nil
	}

	return d.HasPermission(PermissionAdministrator, inclusive)
}
