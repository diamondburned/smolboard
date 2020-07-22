package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
)

func TestPostNew(t *testing.T) {
	p, err := NewEmptyPost("image/png")
	if err != nil {
		t.Fatal("Failed to make empty post:", err)
	}

	if name := p.Filename(); name != fmt.Sprintf("%d.png", p.ID) {
		t.Fatal("Unexpected filename:", name)
	}

	t.Run("InvalidContentType", func(t *testing.T) {
		var contentTypes = []string{
			"audio/ogg",
			"",
			"image/",
			"image/tiff",
		}

		for _, ctype := range contentTypes {
			if _, err := NewEmptyPost(ctype); !errors.Is(err, ErrUnsupportedFileType) {
				t.Fatalf("Unexpected content-type (%q) error: %v", ctype, err)
			}
		}
	})
}

func TestPost(t *testing.T) {
	d := newTestDatabase(t)

	ownerToken := testNewOwnerToken(t, d, "ひめありかわ", "password")

	newUser := func(t *testing.T, name string, p Permission) *Transaction {
		token := testOneTimeToken(t, d, ownerToken)

		u, err := d.Signup(context.TODO(), name, "password", token, "A")
		if err != nil {
			t.Fatal("Failed to create normal user:", err)
		}

		// Start an owner Tx and manually promote.
		err = d.Acquire(context.TODO(), ownerToken, func(tx *Transaction) error {
			return tx.PromoteUser(u.Username, p)
		})

		if err != nil {
			t.Fatal("Failed to promote user:", err)
		}

		return testBeginTx(t, d, u.AuthToken)
	}

	var tests = []struct {
		name      string
		begin     func(t *testing.T) *Transaction
		passPerms []Permission
		failPerms []Permission
	}{
		{
			"Owner", func(t *testing.T) *Transaction {
				return testBeginTx(t, d, ownerToken)
			},
			[]Permission{
				PermissionOwner,
				PermissionAdministrator,
				PermissionTrusted,
				PermissionOwner,
			},
			[]Permission{},
		},
		{
			"Regular", func(t *testing.T) *Transaction {
				return newUser(t, "かぐやありかわ", PermissionNormal)
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
		{
			"Trusted", func(t *testing.T) *Transaction {
				return newUser(t, "おだみつなが", PermissionTrusted)
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
		{
			"Administrator", func(t *testing.T) *Transaction {
				return newUser(t, "ひろ", PermissionAdministrator)
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

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			tx := test.begin(t)

			p, err := NewEmptyPost("image/png")
			if err != nil {
				t.Fatal("Failed to create post:", err)
			}

			t.Run("Save", func(t *testing.T) {
				if err := tx.SavePost(&p); err != nil {
					t.Fatal("Failed to save post:", err)
				}
			})

			t.Run("PassPermissions", func(t *testing.T) {
				for _, perm := range test.passPerms {
					if err := tx.SetPostPermission(p.ID, perm); err != nil {
						t.Fatalf("Failed to set post to permission %v: %v", perm, err)
					}
				}
			})

			t.Run("FailPermissions", func(t *testing.T) {
				for _, perm := range test.failPerms {
					err := tx.SetPostPermission(p.ID, perm)
					if !errors.Is(err, ErrActionNotPermitted) {
						t.Fatalf("Unexpected error while trying fail perm %v: %v", perm, err)
					}
				}
			})

			t.Run("Delete", func(t *testing.T) {
				if err := tx.DeletePost(p.ID); err != nil {
					t.Fatal("Failed to delete post:", err)
				}
			})
		})
	}
}
