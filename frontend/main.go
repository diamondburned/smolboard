package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/diamondburned/smolboard/frontend/frontserver"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

type Config struct {
	ListenAddress  string `toml:"listenAddress"`
	BackendAddress string `toml:"backendAddress"`
	frontserver.FrontConfig
}

func NewConfig() Config {
	return Config{
		FrontConfig: frontserver.NewConfig(),
	}
}

func (c *Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("Field `listenAddress' missing")
	}
	if c.BackendAddress == "" {
		return errors.New("Field `backendAddress missing'")
	}

	_, err := url.Parse(c.BackendAddress)
	if err != nil {
		return errors.Wrap(err, "Failed to parse value of `backendAddress'")
	}

	return c.FrontConfig.Validate()
}

var (
	configGlob = "./config*.toml"
)

func init() {
	pflag.StringVarP(
		&configGlob, "config", "c", configGlob,
		"Path to config file with glob support for fallback",
	)
}

func main() {
	pflag.Parse()

	// Read all globs.
	d, err := filepath.Glob(configGlob)
	if err != nil {
		log.Fatalln("Failed to glob:", err)
	}

	if len(d) == 0 {
		log.Fatalln("Glob returns no matches.")
	}

	var cfg = NewConfig()

	for _, path := range d {
		f, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalln("Failed to read globbed config file:", err)
		}

		if err := toml.Unmarshal(f, &cfg); err != nil {
			log.Fatalln("Failed to unmarshal from TOML:", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalln("Config error:", err)
	}

	f, err := frontserver.NewWithHTTPBackend(cfg.BackendAddress, cfg.FrontConfig)
	if err != nil {
		log.Fatalln("Failed to create frontend:", err)
	}

	log.Println("Listening to", cfg.ListenAddress)

	if err := http.ListenAndServe(cfg.ListenAddress, f); err != nil {
		log.Fatalln("Failed to listen/serve:", err)
	}
}
