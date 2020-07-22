package db

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestSession(t *testing.T) {
	d := newTestDatabase(t)

	t.Log("Created database. Hashing owner password...")

	// Password hashing takes dummy long.
	ownerToken := testNewOwnerToken(t, d, "ひめありかわ", "goodpassword")
	t.Log("Made owner token:", ownerToken)

	token := testOneTimeToken(t, d, ownerToken)
	t.Log("Made one-timer invitation token:", token)

	t.Log("Initialized one timer token. Hashing new user password...")

	s, err := d.Signup(context.Background(), "かぐやありかわ", "abcd1234", token, "iOS")
	if err != nil {
		t.Fatal("Failed to create a new user:", err)
	}

	t.Log("Signed up as", s.Username)

	tx, err := d.begin(context.Background(), s.AuthToken)
	if err != nil {
		t.Fatal("Failed to get newly created session:", err)
	}

	t.Log("Transaction has begun.")

	n := tx.Session()

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

	sessions, err := tx.Sessions()
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

	if err := tx.Signout(); err != nil {
		t.Fatal("Failed to delete the new session:", err)
	}

	t.Log("Signed out.")

	// Close the transaction.
	if err := tx.Commit(); err != nil {
		t.Fatal("Failed to commit:", err)
	}

	t.Log("Committed.")

	// Try and start a session with a signed out token.
	_, err = d.begin(context.Background(), n.AuthToken)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("Unexpected error looking for deleted session:", err)
	}
}
