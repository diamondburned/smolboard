package smolboard

import (
	"github.com/diamondburned/smolboard/smolboard/db"
	"github.com/diamondburned/smolboard/smolboard/http"
	"github.com/pkg/errors"
)

// Config is the global application config.
type Config struct {
	db.DBConfig
	http.HTTPConfig
}

func NewConfig() Config {
	return Config{
		DBConfig:   db.NewConfig(),
		HTTPConfig: http.NewConfig(),
	}
}

// Validator is used for configs.
type Validator interface {
	Validate() error
}

func (c *Config) Validate() error {
	var fields = []Validator{
		&c.DBConfig,
		&c.HTTPConfig,
	}

	for _, v := range fields {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func CreateOwner(config Config, password []byte) error {
	return db.CreateOwner(config.DBConfig, string(password))
}

type App struct {
	*http.Routes
	Database *db.Database
}

func ListenAndServe(config Config) error {
	a, err := New(config)
	if err != nil {
		return err
	}

	return a.ListenAndServe()
}

func New(config Config) (*App, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	d, err := db.NewDatabase(config.DBConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create database")
	}

	h, err := http.New(d, config.HTTPConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create HTTP")
	}

	app := &App{
		Routes:   h,
		Database: d,
	}

	return app, nil
}
