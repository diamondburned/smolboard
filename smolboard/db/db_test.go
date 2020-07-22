package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

var _testdb uint64 = 4

func newTestDatabase(t *testing.T) *Database {
	t.Helper()

	// Get the current unique index.
	u := atomic.AddUint64(&_testdb, 1)

	var dbpath = filepath.Join(
		os.TempDir(),
		fmt.Sprintf("smolboard-db-test-%d-%d", time.Now().UnixNano(), u),
	)

	// Remove the database before the testing.
	os.Remove(dbpath)
	// Remove the database after the testing.
	t.Cleanup(func() { os.Remove(dbpath) })

	cfg := NewConfig()
	cfg.Owner = "ひめありかわ1"
	cfg.DatabasePath = dbpath

	// Start a fresh database.
	d, err := NewDatabase(cfg)
	if err != nil {
		t.Fatal("Failed to create a database:", err)
	}

	t.Cleanup(func() { d.Close() })

	return d
}

func testNewOwnerToken(t *testing.T, db *Database, user, pass string) string {
	t.Helper()

	db.Config.Owner = user

	if err := db.createOwner(pass); err != nil {
		t.Fatal("Failed to make owner user:", err)
	}

	k, err := db.Signin(context.Background(), user, pass, "GoTest")
	if err != nil {
		t.Fatal("Failed to sign in:", err)
	}

	return k.AuthToken
}

func testBeginTx(t *testing.T, db *Database, token string) *Transaction {
	t.Helper()

	tx, err := db.begin(context.Background(), token)
	if err != nil {
		t.Fatal("Failed to begin transaction:", err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(); err != nil {
			tx.Rollback()
			t.Fatal("Failed to commit:", err)
		}
	})

	return tx
}

// testOneTimeToken generates a new token. It begins and ends a transaction
// on its own.
func testOneTimeToken(t *testing.T, db *Database, token string) string {
	t.Helper()

	tx, err := db.begin(context.Background(), token)
	if err != nil {
		t.Fatal("Failed to begin transaction:", err)
	}
	defer tx.Rollback()

	// We need a token.
	k := testMustOneTimeToken(t, tx)

	if err := tx.Commit(); err != nil {
		t.Fatal("Failed to commit:", err)
	}

	return k
}

func testMustOneTimeToken(t *testing.T, tx *Transaction) string {
	k, err := tx.CreateToken(1)
	if err != nil {
		t.Fatal("Failed to create token:", err)
	}
	return k.Token
}

func TestDatabase(t *testing.T) {
	newTestDatabase(t)
}
