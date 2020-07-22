package starboard

import (
	"github.com/diamondburned/smolboard/smolboard/db"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

// Config is the global application config.
type Config struct {
	HTTPAddr string `ini:"http_address"`
	db.Config
}

func NewConfig() Config {
	return Config{
		Config: db.NewConfig(),
	}
}

func (c *Config) Validate() error {
	if c.HTTPAddr == "" {
		c.HTTPAddr = ":0"
	}

	return c.Config.Validate()
}

type App struct {
	*chi.Mux // go-chi gang go-chi gang

	store *db.Database
}

func New(config Config) (*App, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	s, err := db.NewDatabase(config.Config)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create database")
	}

	app := &App{
		Mux:   chi.NewMux(),
		store: s,
	}

	return app, nil
}
