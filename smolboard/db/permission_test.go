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

	var s *Session

	err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
		s, err = tx.Signup(name, "password", token, "iOS")
		return
	})

	if err != nil {
		t.Fatal("Failed to create normal user:", err)
	}

	err = d.Acquire(context.TODO(), ownerToken, func(tx *Transaction) error {
		return tx.PromoteUser(s.Username, p)
	})

	if err != nil {
		t.Fatal("Failed to promote user:", err)
	}

	return s
}

var testPermissionSet = map[Permission]permTest{
	PermissionOwner: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return owner
		},
		[]Permission{
			PermissionGuest,
			PermissionUser,
			PermissionTrusted,
			PermissionAdministrator,
			PermissionOwner,
		},
		[]Permission{},
	},
	PermissionUser: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return newTestUser(t, d, owner.AuthToken, "かぐやありかわ", PermissionUser)
		},
		[]Permission{
			PermissionGuest,
			PermissionUser,
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
			PermissionGuest,
			PermissionUser,
			PermissionTrusted,
		},
		[]Permission{
			PermissionOwner,
			PermissionAdministrator,
		},
	},
	PermissionAdministrator: {
		func(t *testing.T, d *Database, owner *Session) *Session {
			t.Helper()
			return newTestUser(t, d, owner.AuthToken, "とよとみヒロ", PermissionAdministrator)
		},
		[]Permission{
			PermissionGuest,
			PermissionUser,
			PermissionTrusted,
			PermissionAdministrator,
		},
		[]Permission{
			PermissionOwner,
		},
	},
}
