package main

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/spf13/pflag"

	toml "github.com/pelletier/go-toml"
)

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

	var cfg = smolboard.NewConfig()

	for _, path := range d {
		f, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatalln("Failed to read globbed config file:", err)
		}

		if err := toml.Unmarshal(f, &cfg); err != nil {
			log.Fatalln("Failed to unmarshal from TOML:", err)
		}
	}

	log.Println("Starting listener at", cfg.Address)

	if err := smolboard.ListenAndServe(cfg); err != nil {
		log.Fatalln("Failed to start:", err)
	}
}
