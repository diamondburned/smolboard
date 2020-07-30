package render

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/diamondburned/smolboard/client"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
)

// Renderer represents a renderable page.
type Renderer = func(r Request) (Render, error)

type Render struct {
	Title       string // og:title, <title>
	Description string // og:description
	ImageURL    string // og:image

	Body template.HTML
}

type Config struct {
	SiteName string `toml:"siteName"`
}

func NewConfig() Config {
	return Config{
		SiteName: "smolboard",
	}
}

func (c *Config) Validate() error {
	return nil
}

type renderCtx struct {
	Theme  Theme
	Render Render
	Config Config
}

func (r renderCtx) FormatTitle() string {
	if r.Render.Title == "" {
		return r.Config.SiteName
	}
	return fmt.Sprintf("%s - %s", r.Render.Title, r.Config.SiteName)
}

type Request struct {
	*http.Request
	writer FlushWriter
	pusher http.Pusher

	Config    Config
	Session   *client.Session
	cookieURL *url.URL
}

// FlushCookies flushes the cookies.
func (r *Request) FlushCookies() {
	r.Session.Client.Jar.Cookies(r.cookieURL)
}

func (r *Request) Push(url string) {
	if r.pusher == nil {
		ps, ok := r.writer.(http.Pusher)
		if !ok {
			return
		}
		r.pusher = ps
	}

	r.pusher.Push(url, nil)
}

func pushAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pusher, ok := w.(http.Pusher); ok {
			// Push over the CSS to save a trip.
			pusher.Push("/static/components.css", nil)

			// Push over all the theme CSS.
			for i := Theme(0); i < themeLen; i++ {
				pusher.Push(i.URL(), nil)
			}
		}
	})
}

type Mux struct {
	*chi.Mux
	host string
	cfg  Config
}

func NewMux(serverHost string, cfg Config) *Mux {
	r := chi.NewMux()
	r.Use(ThemeM)
	r.Post("/theme", handleSetTheme)
	r.Route("/static", func(r chi.Router) {
		r.Get("/components.css", componentsCSSHandler)
		r.Mount("/", http.FileServer(pkger.Dir("/frontend/frontserver/static/")))
	})

	return &Mux{r, serverHost, cfg}
}

func (m *Mux) NewRequest(w http.ResponseWriter, r *http.Request) Request {
	url := *r.URL
	url.Host = r.Host

	s := client.NewSession(m.host)
	s.Client.SetCookies(&url, r.Cookies())

	return Request{
		Request:   r,
		writer:    TryFlushWriter(w),
		Session:   s,
		Config:    m.cfg,
		cookieURL: &url,
	}
}

// M is the middleware wrapper.
func (m *Mux) M(render Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Write the proper headers.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		var request = m.NewRequest(w, r)

		page, err := render(request)
		if err != nil {
			// TODO
			log.Println("Error:", err)
			return
		}

		// Flush the cookies before writing the body.
		request.FlushCookies()

		var renderCtx = renderCtx{
			Theme:  GetTheme(r.Context()),
			Render: page,
			Config: m.cfg,
		}

		if err := index.Execute(w, renderCtx); err != nil {
			return
		}
	}
}

func (m *Mux) Get(route string, r Renderer) {
	m.Mux.Get(route, m.M(r))
}
