package db

import (
	"context"
	"testing"

	"github.com/pkg/errors"
)

func TestInclusivePermission(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "password")

	for perm, test := range testPermissionSet {
		t.Run(perm.String(), func(t *testing.T) {
			auth := test.begin(t, d, owner)
			tx := testBeginTx(t, d, auth.AuthToken)

			t.Run("CheckPass", func(t *testing.T) {
				for _, perm := range test.passPerms {
					if err := tx.HasPermOverUser(perm, auth.Username); err != nil {
						t.Fatalf("User erroneously does not have permission %v: %v", perm, err)
					}
				}
			})

			t.Run("CheckFail", func(t *testing.T) {
				for _, perm := range test.failPerms {
					err := tx.HasPermOverUser(perm, auth.Username)
					if !errors.Is(err, ErrActionNotPermitted) {
						t.Fatalf("User erroneously does have permission %v: %v", perm, err)
					}
				}
			})
		})
	}
}

type permTest struct {
	begin     func(t *testing.T, d *Database, owner *Session) *Session
	passPerms []Permission
	failPerms []Permission
}

func newTestUser(t *testing.T, d *Database, ownerToken, name string, p Permission) *Session {
	t.Helper()

	token := testOneTimeToken(t, d, ownerToken)

	u, err := d.Signup(context.TODO(), name, "password", token, "A")
	if err != nil {
		t.Fatal("Failed to create normal user:", err)
	}

	err = d.Acquire(context.TODO(), ownerToken, func(tx *Transaction) error {
		return tx.PromoteUser(u.Username, p)
	})

	if err != nil {
		t.Fatal("Failed to promote user:", err)
	}

	return u
}

var testPermissionSet = map[Permission]permTest{
	PermissionOwner: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return owner
		},
		[]Permission{
			PermissionOwner,
			PermissionAdministrator,
			PermissionTrusted,
			PermissionOwner,
		},
		[]Permission{},
	},
	PermissionNormal: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return newTestUser(t, d, owner.AuthToken, "かぐやありかわ", PermissionNormal)
		},
		[]Permission{
			PermissionNormal,
		},
		[]Permission{
			PermissionOwner,
			PermissionAdministrator,
			PermissionTrusted,
		},
	},
	PermissionTrusted: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return newTestUser(t, d, owner.AuthToken, "みつながおだ", PermissionTrusted)
		},
		[]Permission{
			PermissionNormal,
			PermissionTrusted,
		},
		[]Permission{
			PermissionAdministrator,
			PermissionOwner,
		},
	},
	PermissionAdministrator: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return newTestUser(t, d, owner.AuthToken, "とよとみヒロ", PermissionAdministrator)
		},
		[]Permission{
			PermissionNormal,
			PermissionTrusted,
			PermissionAdministrator,
		},
		[]Permission{
			PermissionOwner,
		},
	},
}
