package db

import (
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestSignup(t *testing.T) {
	d := newTestDatabase(t)

	k, err := d.CreateToken(1)
	if err != nil {
		t.Fatal("Failed to create a one-timer token:", err)
	}

	s, err := d.Signup("ひめありかわ1", "password", k.Token, "Android")
	if err != nil {
		t.Fatal("Failed to signup:", err)
	}

	if err := d.Signout(s.AuthToken); err != nil {
		t.Fatal("Failed to sign out:", err)
	}

	_, err = d.Signin("ひめありかわ1", "passwrod", "iOS")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("Unexpected error with invalid password:", err)
	}

	n, err := d.Signin("ひめありかわ1", "password", "curl")
	if err != nil {
		t.Fatal("Failed to sign in:", err)
	}

	if n.ID == s.ID || n.AuthToken == s.AuthToken {
		t.Fatal("New session contains matching elements from old token.")
	}

	if n.Deadline <= s.Deadline {
		t.Fatal("New session's deadline is before old session's deadline.")
	}
}

func TestSession(t *testing.T) {
	d := newTestDatabase(t)

	u, err := d.NewUser("ひめありかわ1", "abcd1234", PermissionNormal)
	if err != nil {
		t.Fatal("Failed to create a new user:", err)
	}

	s, err := d.newSession(u.Username, "Android")
	if err != nil {
		t.Fatal("Failed to create a new session:", err)
	}

	n, err := d.Session(s.AuthToken)
	if err != nil {
		t.Fatal("Failed to get newly created session:", err)
	}

	if s.Deadline >= n.Deadline {
		t.Fatal("Fetched session's deadline is not renewed.")
	}

	if s.AuthToken != n.AuthToken {
		t.Fatal("Fetched session has mismatched token.")
	}

	if s.ID != n.ID {
		t.Fatal("Fetched session has mismatched ID.")
	}

	// Make sure the deadline is in the future.
	if time.Now().UnixNano() > n.Deadline {
		t.Fatal("Fetched session has deadline already passed.")
	}

	// Get the list of sessions and find manually.
	sessions, err := d.Sessions(u.Username)
	if err != nil {
		t.Fatal("Failed to get all sessions:", err)
	}

	if len(sessions) != 1 {
		t.Fatal("Unexpected sessions returned:", len(sessions))
	}

	var found bool

	for _, session := range sessions {
		if session.ID == s.ID {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Session not found in list: %#v", sessions)
	}

	// Try and promote this user.
	if err := d.SetUserPermission(n.Username, PermissionTrusted); err != nil {
		t.Fatal("Failed to promote user:", err)
	}

	if err := d.Signout(n.AuthToken); err != nil {
		t.Fatal("Failed to delete the session:", err)
	}

	if _, err := d.Session(n.AuthToken); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("Unexpected error looking for deleted session:", err)
	}
}
