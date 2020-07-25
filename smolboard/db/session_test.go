package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestSession(t *testing.T) {
	d := newTestDatabase(t)

	t.Log("Created database. Hashing owner password...")

	// Password hashing takes dummy long with the race detector.
	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")
	ownerToken := owner.AuthToken
	t.Log("Made owner token:", ownerToken)

	token := testOneTimeToken(t, d, ownerToken)
	t.Log("Made one-timer invitation token:", token)

	t.Log("Initialized one timer token. Hashing new user password...")

	var s *Session

	err := d.AcquireGuest(context.Background(), func(tx *Transaction) (err error) {
		s, err = tx.Signup("かぐやありかわ", "abcd1234", token, "iOS")
		return
	})

	if err != nil {
		t.Fatal("Failed to sign up:", err)
	}

	t.Log("Signed up as", s.Username)

	tx, err := d.begin(context.Background(), s.AuthToken)
	if err != nil {
		t.Fatal("Failed to get newly created session:", err)
	}

	t.Log("Transaction has begun.")

	n := tx.Session

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
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatal("Unexpected error looking for deleted session:", err)
	}
}

func TestSessionDelete(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	tx := testBeginTx(t, d, owner.AuthToken)

	if err := tx.DeleteSessionID(owner.ID); err != nil {
		t.Fatal("Failed to delete owner's session:", err)
	}
}

func TestSessionSignout(t *testing.T) {
	d := newTestDatabase(t)

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	tx := testBeginTx(t, d, owner.AuthToken)

	if err := tx.Signout(); err != nil {
		t.Fatal("Failed to sign out:", err)
	}

	// Re-signout.
	if err := tx.Signout(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("Unexpected error signing out of signed out session:", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	d := newTestDatabase(t)
	d.Config.tokenLifespan = 200 * time.Millisecond

	owner := testNewOwner(t, d, "ひめありかわ", "goodpassword")

	tx := testBeginTx(t, d, owner.AuthToken)

	s, err := tx.newSession("ひめありかわ", "A")
	if err != nil {
		t.Fatal("Failed to create session:", err)
	}

	// Wait for session to expire.
	<-time.After(d.Config.tokenLifespan)

	// Query.
	_, err = tx.querySession(s.AuthToken)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatal("Unexpected error querying expired session:", err)
	}

	// Cleanup.
	if err := tx.cleanupSession(time.Now().UnixNano()); err != nil {
		t.Fatal("Failed to cleanup session:", err)
	}

	// Check.
	_, err = tx.querySession(s.AuthToken)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatal("Unexpected error querying expired session:", err)
	}

	// Check manually.
	err = tx.
		QueryRow("SELECT id FROM sessions WHERE id = ?", s.ID).
		Scan(new(int64))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("Session still found post-cleanup:", err)
	}

	// Try deleting.
	if err := tx.DeleteSessionID(s.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("Unexpected error while deleting expired session:", err)
	}
}
