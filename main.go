package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/diamondburned/smolboard/server"
	"github.com/go-chi/chi"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"

	toml "github.com/pelletier/go-toml"
)

var (
	configGlob = "./config*.toml"
)

func stderrlnf(f string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
}

type Config struct {
	Address string `toml:"address"`
	server.Config
}

func NewConfig() Config {
	return Config{
		Config: server.NewConfig(),
	}
}

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

	switch pflag.Arg(0) {
	case "create-owner":
		fmt.Print("Enter your password: ")
		p, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatalln("Failed to read password:", err)
		}

		fmt.Println()

		if err := server.CreateOwner(cfg.Config, p); err != nil {
			log.Fatalln(err)
		}

	case "serve":
		fallthrough
	default:
		a, err := server.New(cfg.Config)
		if err != nil {
			log.Fatalln("Failed to create instance:", err)
		}

		if cfg.Address == "" {
			log.Fatalln("Missing field `address'")
		}

		// Build wasm if possible.
		if s, err := os.Stat("frontend/"); err == nil && s.IsDir() {
			log.Println("Building wasm frontend...")

			sh(`cd frontend/src && \
				go generate     && \
				go build -o ../bin/main.wasm .`)
		}

		mux := chi.NewMux()
		mux.Mount("/api/v1", a)
		mux.Mount("/", http.FileServer(http.Dir("./frontend/bin")))

		log.Println("Starting listener at", cfg.Address)

		if err := http.ListenAndServe(cfg.Address, mux); err != nil {
			log.Fatalln("Failed to start:", err)
		}
	}
}

func sh(cmd string) {
	c := exec.Command("sh", "-c", cmd)
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	c.Env = append(os.Environ(),
		"GOOS=js",
		"GOARCH=wasm",
	)

	if err := c.Run(); err != nil {
		log.Fatalln(err)
	}
}
