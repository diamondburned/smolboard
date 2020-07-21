package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

var _testdb uint64 = 0

func NewTestDatabase(t *testing.T) *Database {
	return newTestDatabase(t)
}

func newTestDatabase(t *testing.T) *Database {
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
	cfg.DatabasePath = dbpath

	// Start a fresh database.
	d, err := NewDatabase(cfg)
	if err != nil {
		t.Fatal("Failed to create a database:", err)
	}

	t.Cleanup(func() { d.Close() })

	return d
}

func TestDatabase(t *testing.T) {
	newTestDatabase(t)
}
