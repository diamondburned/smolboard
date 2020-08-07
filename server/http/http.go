package http

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/db"
	"github.com/diamondburned/smolboard/server/http/internal/limit"
	"github.com/diamondburned/smolboard/server/http/internal/limread"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/diamondburned/smolboard/server/http/post"
	"github.com/diamondburned/smolboard/server/http/token"
	"github.com/diamondburned/smolboard/server/http/upload"
	"github.com/diamondburned/smolboard/server/http/upload/imgsrv"
	"github.com/diamondburned/smolboard/server/http/user"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

type HTTPConfig struct {
	MaxBodySize datasize.ByteSize `toml:"maxBodySize"`
	// inherit upload's config
	upload.UploadConfig
}

func NewConfig() HTTPConfig {
	return HTTPConfig{
		MaxBodySize:  1 * datasize.GB,
		UploadConfig: upload.NewConfig(),
	}
}

func (c *HTTPConfig) Validate() error {
	return c.UploadConfig.Validate()
}

func GetTypes(cfg HTTPConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(cfg.AllowedTypes); err != nil {
			log.Println("Encode failed:", err)
		}
	}
}

type Routes struct {
	http.Handler
	mw  tx.Middleware
	cfg HTTPConfig
}

func New(db *db.Database, cfg HTTPConfig) (*Routes, error) {
	mux := chi.NewMux()
	rts := &Routes{
		Handler: mux,
		mw:      tx.NewMiddleware(db, cfg.UploadConfig),
		cfg:     cfg,
	}

	// Alias the middleware function.
	m := rts.mw.M

	mux.Use(
		middleware.RealIP,
		middleware.Recoverer,
		middleware.Compress(5),
		limread.LimitBody(cfg.MaxBodySize),
	)

	mux.Group(func(mux chi.Router) {
		mux.Use(limit.RateLimit(2))
		mux.Post("/signin", m(user.Signin))
		mux.Post("/signup", m(user.Signup))
		mux.Post("/signout", m(user.Signout))
	})

	mux.Group(func(mux chi.Router) {
		mux.Use(limit.RateLimit(64))
		mux.Get("/filetypes", GetTypes(cfg))
	})

	mux.Mount("/tokens", token.Mount(m))
	mux.Mount("/images", imgsrv.Mount(m))
	mux.Mount("/posts", post.Mount(m))
	mux.Mount("/users", user.Mount(m))

	return rts, nil
}
