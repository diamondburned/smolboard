package session

import (
	"errors"

	"github.com/diamondburned/smolboard/smolboard/session/internal/db"
)

type Config struct {
	Owner string `ini:"owner"`
	db.Config
}

func NewConfig() Config {
	return Config{
		Owner:  "",
		Config: db.NewConfig(),
	}
}

func (c *Config) Validate() error {
	if c.Owner == "" {
		return errors.New("Missing `owner' value")
	}

	return c.Config.Validate()
}

type Database struct {
	*db.Database
	Config Config
}

func NewDatabase(config Config) (*Database, error) {
	d, err := db.NewDatabase(config.Config)
	if err != nil {
		return nil, err
	}

	return &Database{d, config}, nil
}

func (d *Database) Signin(username, password, userAgent string) (*Actor, error) {
	t, err := d.Database.Signin(username, password, userAgent)
	if err != nil {
		return nil, err
	}

	return &Actor{database: d, authtoken: t}, nil
}

func (d *Database) Signup(username, password, token, userAgent string) (*Actor, error) {
	t, err := d.Database.Signup(username, password, token, userAgent)
	if err != nil {
		return nil, err
	}

	return &Actor{database: d, authtoken: t}, nil
}
