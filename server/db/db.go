package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/diamondburned/duration"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

var migrations = []string{`
	CREATE TABLE users (
		username   TEXT    PRIMARY KEY,
		passhash   BLOB    NOT NULL, -- bcrypt probably
		permission INTEGER NOT NULL  -- Permission enum
	);

	CREATE TABLE tokens (
		token   TEXT NOT NULL,
		creator TEXT NOT NULL REFERENCES users(username)
			ON UPDATE CASCADE
			ON DELETE CASCADE,
		remaining INTEGER NOT NULL -- (-1) for unlimited, owner only
	);

	CREATE TABLE sessions (
		id       INTEGER PRIMARY KEY,
		username TEXT REFERENCES users(username)
			ON UPDATE CASCADE
			ON DELETE CASCADE,
		authtoken TEXT    NOT NULL UNIQUE,
		deadline  INTEGER NOT NULL, -- unixnano
		useragent TEXT    NOT NULL
	);

	CREATE TABLE posts (
		id     INTEGER PRIMARY KEY, -- Snowflake
		poster TEXT REFERENCES users(username)
			ON UPDATE CASCADE
			ON DELETE SET NULL,
		contenttype TEXT    NOT NULL,
		permission  INTEGER NOT NULL,
		attributes  BLOB
	);

	CREATE TABLE posttags (
		postid  INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
		tagname TEXT    NOT NULL,
		-- Prevent multiple of the same tags from appearing in one post.
		UNIQUE (postid, tagname)
	);
`}

type DBConfig struct {
	Owner         string `toml:"owner"`
	DatabasePath  string `toml:"databasePath"`
	MaxTokenUses  int    `toml:"maxTokenUses"`
	TokenLifespan string `toml:"tokenLifespan"`

	tokenLifespan time.Duration
}

func NewConfig() DBConfig {
	return DBConfig{
		MaxTokenUses:  100,
		TokenLifespan: "7d",
	}
}

func (c *DBConfig) Validate() error {
	if c.Owner == "" {
		return errors.New("missing `owner' value")
	}

	if c.DatabasePath == "" {
		return errors.New("missing `databasePath' value")
	}

	d, err := duration.ParseDuration(c.TokenLifespan)
	if err != nil {
		return errors.Wrap(err, "invalid token lifespan")
	}
	c.tokenLifespan = time.Duration(d)

	return nil
}

type Database struct {
	*sqlx.DB
	Config DBConfig
}

func NewDatabase(config DBConfig) (*Database, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	d, err := sqlx.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open sqlite3 db")
	}

	db := &Database{d, config}

	// Enable foreign key constraints.
	if err := db.enableFK(); err != nil {
		return nil, errors.Wrap(err, "Failed to enable foreign key constraints")
	}

	v, err := db.userVersion()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get user_version pragma")
	}

	// If we're already up-to-date with all the migrations, then we're done.
	if v >= len(migrations) {
		return db, nil
	}

	// Start a transaction because yadda yadda speed.
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to start a transaction for migrations")
	}
	// Rollback in the end even if we've failed, just in case.
	defer tx.Rollback()

	// Handle migrations. We just pick up from the changes in the migrations
	// slice.
	for i := v; i < len(migrations); i++ {
		_, err := tx.Exec(migrations[i])
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to migrate at step %d", i)
		}
	}

	// Save the version.
	if err := db.setUserVersion(tx, len(migrations)); err != nil {
		return nil, errors.Wrap(err, "Failed to save user_version pragma")
	}

	// Save all changes.
	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "Failed to save migration changes")
	}

	return db, nil
}

// CreateOwner initializes the database once then creates the owner account.
func CreateOwner(config DBConfig, password string) error {
	d, err := NewDatabase(config)
	if err != nil {
		return errors.Wrap(err, "Failed to initialize database")
	}

	if err := d.createOwner(password); err != nil {
		if errIsConstraint(err) {
			return errors.New("owner account already created")
		}
		return errors.Wrap(err, "Failed to create owner")
	}

	return nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) userVersion() (int, error) {
	var version int
	return version, d.QueryRow("PRAGMA user_version").Scan(&version)
}

func (d *Database) setUserVersion(tx *sql.Tx, v int) error {
	_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
	return err
}

func (d *Database) enableFK() error {
	_, err := d.Exec("PRAGMA foreign_keys = ON")
	return err
}

// createOwner is an internal function.
func (d *Database) createOwner(password string) error {
	return d.AcquireGuest(context.Background(), func(tx *Transaction) error {
		return tx.createUser(d.Config.Owner, password, smolboard.PermissionOwner)
	})
}

// Transaction acquires a transaction lock in SQLite. This bugs me so much. Why
// did I do this? I guess you could say that it boils down to not having any
// data race at all. But think about this scenario: if 2 viewers access the
// webpage within the same millisecond, then one would have to wait a few
// additional milliseconds. This BUGS ME!!!! WHY!!!! WHY DID I DO THIS?!
type Transaction struct {
	*sqlx.Tx

	// As we acquire an entire transaction, it is safe to store our own local
	// session state as long as we keep it up to date on our own calls.
	Session smolboard.Session
	config  DBConfig
}

func (d *Database) begin(ctx context.Context, session string) (*Transaction, error) {
	t, err := d.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}

	tx := &Transaction{
		Tx:     t,
		config: d.Config,
	}

	// Verify session.
	s, err := tx.querySession(session)
	if err != nil {
		return nil, err
	}

	tx.Session = *s
	return tx, nil
}

type TxHandler = func(*Transaction) error

func (d *Database) Acquire(ctx context.Context, session string, fn TxHandler) error {
	t, err := d.begin(ctx, session)
	if err != nil {
		return errors.Wrap(err, "Failed to begin transaction")
	}
	defer t.Rollback()

	if err := fn(t); err != nil {
		return err
	}

	return t.Commit()
}

var readOnlyOpts = &sql.TxOptions{
	Isolation: sql.LevelSnapshot,
	ReadOnly:  true,
}

func (d *Database) AcquireGuest(ctx context.Context, fn TxHandler) error {
	t, err := d.DB.BeginTxx(ctx, readOnlyOpts)
	if err != nil {
		return errors.Wrap(err, "Failed to begin transaction")
	}
	defer t.Rollback()

	tx := &Transaction{
		Tx:     t,
		config: d.Config,
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func errIsConstraint(err error) bool {
	if err != nil {
		sqlerr := sqlite3.Error{}

		// Unique constraint means we're attempting to make a username that's
		// colliding. We could return an error close to that.
		if errors.As(err, &sqlerr) && sqlerr.Code == sqlite3.ErrConstraint {
			return true
		}
	}

	return false
}

func (d *Transaction) execChanged(exec string, v ...interface{}) (bool, error) {
	return execChanged(d.Tx.Tx, exec, v...)
}

// execChanged returns false if no rows were affected.
func execChanged(tx *sql.Tx, exec string, v ...interface{}) (bool, error) {
	// Ensure that we are deleting only this user's token.
	r, err := tx.Exec(exec, v...)
	if err != nil {
		return false, errors.Wrap(err, "Failed to delete token")
	}

	count, err := r.RowsAffected()
	if err != nil {
		return false, errors.Wrap(err, "Failed to get rows affected")
	}

	return count > 0, nil
}
