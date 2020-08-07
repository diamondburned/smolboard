package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/diamondburned/smolboard/frontend/frontserver"
	"github.com/diamondburned/smolboard/server"
	"github.com/diamondburned/smolboard/server/http/upload"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/http2"

	toml "github.com/pelletier/go-toml"
)

var (
	configGlob = "./config*.toml"
	noFrontend = false
)

func stderrlnf(f string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, f+"\n", v...)
}

type Config struct {
	SocketPath string `toml:"socketPath"`
	SocketPerm string `toml:"socketPerm"`

	frontserver.FrontConfig
	server.Config
}

func NewConfig() Config {
	return Config{
		FrontConfig: frontserver.NewConfig(),
		Config:      server.NewConfig(),
	}
}

func init() {
	pflag.StringVarP(
		&configGlob, "config", "c", configGlob,
		"Path to config file with glob support for fallback",
	)

	pflag.BoolVarP(
		&noFrontend, "no-frontend", "n", noFrontend,
		"Disable the default frontend at root",
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

		t, err := toml.LoadBytes(f)
		if err != nil {
			log.Fatalln("Failed to load TOML:")
		}

		// Workaround: Unmarshal really wants a non-nil MaxSize block, else it
		// will panic. We have to manually insert this if it's not there.
		if !t.Has("MaxSize") {
			t.SetPath([]string{"MaxSize"}, upload.MaxSize{})
		}

		if err := t.Unmarshal(&cfg); err != nil {
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

		c := middleware.NewCompressor(5)
		c.SetEncoder("br", func(w io.Writer, level int) io.Writer {
			return brotli.NewWriterLevel(w, level)
		})

		mux := chi.NewMux()
		mux.Use(c.Handler)
		mux.Mount("/api/v1", a)

		if !noFrontend {
			f, err := frontserver.New(cfg.SocketPath, cfg.FrontConfig)
			if err != nil {
				log.Fatalln("Failed to create frontend:", err)
			}
			mux.Mount("/", f)
		}

		// Ensure that the socket is cleaned up because we're not gracefully
		// handling closes.
		if err := os.Remove(cfg.SocketPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Fatalln("Failed to clean up old socket:", err)
			}
		}

		l, err := net.Listen("unix", cfg.SocketPath)
		if err != nil {
			log.Fatalln("Failed to listen to Unix socket:", err)
		}

		if cfg.SocketPerm != "" {
			o, err := strconv.ParseUint(cfg.SocketPerm, 8, 32)
			if err != nil {
				log.Fatalln("Failed to parse socket perm in octet:", err)
			}
			if err := os.Chmod(cfg.SocketPath, os.FileMode(o)); err != nil {
				log.Fatalln("Failed to chmod socket:", err)
			}
		}

		var server = http.Server{
			Handler: mux,
		}

		// Explicitly set up HTTP/2.
		err = http2.ConfigureServer(&server, &http2.Server{
			MaxHandlers:          4096,
			MaxConcurrentStreams: 1024,
		})

		if err != nil {
			log.Fatalln("Failed to configure HTTP/2 server:", err)
		}

		log.Println("Starting HTTP/2 listener at socket", l.Addr())

		go func() {
			if err := server.Serve(l); err != nil {
				log.Fatalln("Failed to start:", err)
			}
		}()

		// Handle SIGINT and gracefully close the server.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig

		// Give the server a 10 seconds timeout for shutting down.
		ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalln("Failed to gracefully close the server:", err)
		}
	}
}
