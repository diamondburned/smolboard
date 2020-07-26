package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"

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

	pflag.Usage = func() {
		stderrlnf("Usage: %s [subcommand] [flags...]", filepath.Base(os.Args[0]))
		stderrlnf("Subcommands:")
		stderrlnf("  create-owner   Initialize a new owner user once")
		stderrlnf("  serve          Run the HTTP server")
		stderrlnf("Flags:")
		pflag.PrintDefaults()
	}
}

func stderrlnf(f string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
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

	switch pflag.Arg(0) {
	case "create-owner":
		fmt.Print("Enter your password: ")
		p, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatalln("Failed to read password:", err)
		}

		fmt.Println()

		if err := smolboard.CreateOwner(cfg, p); err != nil {
			log.Fatalln(err)
		}

	case "serve":
		fallthrough
	default:
		log.Println("Starting listener at", cfg.Address)

		if err := smolboard.ListenAndServe(cfg); err != nil {
			log.Fatalln("Failed to start:", err)
		}
	}
}
