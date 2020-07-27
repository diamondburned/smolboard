package db

import (
	"context"
	"testing"

	"github.com/diamondburned/smolboard/smolboard"
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
					if !errors.Is(err, smolboard.ErrActionNotPermitted) {
						t.Fatalf("User erroneously does have permission %v: %v", perm, err)
					}
				}
			})
		})
	}
}

type permTest struct {
	begin     func(t *testing.T, d *Database, owner *smolboard.Session) *smolboard.Session
	passPerms []smolboard.Permission
	failPerms []smolboard.Permission
}

func newTestUser(
	t *testing.T, d *Database, ownerToken, name string, p smolboard.Permission) *smolboard.Session {

	t.Helper()

	token := testOneTimeToken(t, d, ownerToken)

	var s *smolboard.Session

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

var testPermissionSet = map[smolboard.Permission]permTest{
	smolboard.PermissionOwner: {
		func(t *testing.T, d *Database, owner *smolboard.Session) *smolboard.Session {
			t.Helper()
			return owner
		},
		[]smolboard.Permission{
			smolboard.PermissionGuest,
			smolboard.PermissionUser,
			smolboard.PermissionTrusted,
			smolboard.PermissionAdministrator,
			smolboard.PermissionOwner,
		},
		[]smolboard.Permission{},
	},
	smolboard.PermissionUser: {
		func(t *testing.T, d *Database, owner *smolboard.Session) *smolboard.Session {
			t.Helper()
			return newTestUser(
				t, d, owner.AuthToken, "かぐやありかわ", smolboard.PermissionUser,
			)
		},
		[]smolboard.Permission{
			smolboard.PermissionGuest,
			smolboard.PermissionUser,
		},
		[]smolboard.Permission{
			smolboard.PermissionOwner,
			smolboard.PermissionAdministrator,
			smolboard.PermissionTrusted,
		},
	},
	smolboard.PermissionTrusted: {
		func(t *testing.T, d *Database, owner *smolboard.Session) *smolboard.Session {
			t.Helper()
			return newTestUser(
				t, d, owner.AuthToken, "みつながおだ", smolboard.PermissionTrusted,
			)
		},
		[]smolboard.Permission{
			smolboard.PermissionGuest,
			smolboard.PermissionUser,
			smolboard.PermissionTrusted,
		},
		[]smolboard.Permission{
			smolboard.PermissionOwner,
			smolboard.PermissionAdministrator,
		},
	},
	smolboard.PermissionAdministrator: {
		func(t *testing.T, d *Database, owner *smolboard.Session) *smolboard.Session {
			t.Helper()
			return newTestUser(
				t, d, owner.AuthToken, "とよとみヒロ", smolboard.PermissionAdministrator,
			)
		},
		[]smolboard.Permission{
			smolboard.PermissionGuest,
			smolboard.PermissionUser,
			smolboard.PermissionTrusted,
			smolboard.PermissionAdministrator,
		},
		[]smolboard.Permission{
			smolboard.PermissionOwner,
		},
	},
}
