package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	_ "github.com/mattn/go-sqlite3"
)

var migrations = []string{`
	CREATE TABLE users (
		username   TEXT    PRIMARY KEY,
		passhash   BLOB    NOT NULL, -- bcrypt probably
		permission INTEGER NOT NULL  -- Permission enum
	);

	CREATE TABLE tokens (
		token     TEXT    NOT NULL,
		remaining INTEGER NOT NULL -- (-1) for unlimited, owner only
	);

	CREATE TABLE sessions (
		id       INTEGER PRIMARY KEY,
		username TEXT REFERENCES users(username)
			ON UPDATE CASCADE
			ON DELETE CASCADE,
		authtoken TEXT    NOT NULL,
		deadline  INTEGER NOT NULL, -- unixnano
		useragent TEXT    NOT NULL
	);

	CREATE TABLE posts (
		id    INTEGER PRIMARY KEY, -- Snowflake
		owner INTEGER REFERENCES users(username)
			ON UPDATE CASCADE
			ON DELETE SET NULL,
		fileext    TEXT    NOT NULL,
		permission INTEGER NOT NULL -- canAccess := users(perm) >= posts(perm)
	);

	CREATE TABLE posttags (
		postid  INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
		tagname TEXT    NOT NULL
	);
`}

type Config struct {
	DatabasePath  string `ini:"database_path"`
	MaxTokenUses  int    `ini:"max_token_uses"`
	TokenLifespan string `ini:"token_lifespan"`

	tokenLifespan time.Duration
}

func NewConfig() Config {
	return Config{
		MaxTokenUses:  100,
		TokenLifespan: "1h",
	}
}

func (c *Config) Validate() error {
	// if c.Owner == "" {
	// 	return errors.New("missing `owner' value")
	// }

	if c.DatabasePath == "" {
		return errors.New("missing `database_path' value")
	}

	d, err := time.ParseDuration(c.TokenLifespan)
	if err != nil {
		return errors.Wrap(err, "invalid token lifespan")
	}
	c.tokenLifespan = d

	return nil
}

type Database struct {
	*sqlx.DB
	Config Config

	sf *snowflake.Node
}

func NewDatabase(config Config) (*Database, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	d, err := sqlx.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open sqlite3 db")
	}

	s, err := snowflake.NewNode(0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed ")
	}

	db := &Database{d, config, s}

	v, err := db.userVersion()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get user_version pragma")
	}

	// If we're already up-to-date with all the migrations, then we're done.
	if v >= len(migrations) {
		return db, nil
	}

	// Start a transaction because yadda yadda speed.
	tx, err := db.Begin()
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

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) rowx(v interface{}, q string, args ...interface{}) error {
	return d.QueryRowx(q, args...).Scan(v)
}

func (d *Database) userVersion() (int, error) {
	var version int
	return version, d.QueryRow("PRAGMA user_version").Scan(&version)
}

func (d *Database) setUserVersion(tx *sql.Tx, v int) error {
	_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v))
	return err
}
