package render

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/diamondburned/smolboard/client"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
)

// Renderer represents a renderable page.
type Renderer = func(r *Request) (Render, error)

// ErrorRenderer represents a renderable page for errors.
type ErrorRenderer = func(r *Request, err error) (Render, error)

type Render struct {
	Title       string // og:title, <title>
	Description string // og:description
	ImageURL    string // og:image

	Body template.HTML
}

// Empty is a blank page.
var Empty = Render{}

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
	Writer FlushWriter
	pusher http.Pusher
	CommonCtx
}

// TokenCookie returns the cookie that contains the token if any or nil if
// none.
func (r *Request) TokenCookie() *http.Cookie {
	for _, cookie := range r.Session.Client.Cookies() {
		if cookie.Name == "token" {
			return cookie
		}
	}
	return nil
}

// FlushCookies dumps all the session state's cookies to the response writer.
func (r *Request) FlushCookies() {
	for _, cookie := range r.Session.Client.Cookies() {
		http.SetCookie(r.Writer, cookie)
	}
}

func (r *Request) Push(url string) {
	if r.pusher == nil {
		ps, ok := r.Writer.(http.Pusher)
		if !ok {
			return
		}
		r.pusher = ps
	}

	r.pusher.Push(url, nil)
}

func (r *Request) Param(name string) string {
	return chi.URLParam(r.Request, name)
}

// IDParam returns the ID parameter from chi.
func (r *Request) IDParam() (int64, error) {
	return strconv.ParseInt(r.Param("id"), 10, 64)
}

func (r *Request) Redirect(url string, code int) {
	// Flush the cookies before writing the header.
	r.FlushCookies()
	http.Redirect(r.Writer, r.Request, url, code)
}

type CommonCtx struct {
	Config   Config
	Request  *http.Request
	Session  *client.Session
	Username string
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
	errR ErrorRenderer
}

func NewMux(serverHost string, cfg Config) *Mux {
	r := chi.NewMux()
	r.Use(ThemeM)
	r.Post("/theme", handleSetTheme)
	r.Route("/static", func(r chi.Router) {
		r.Get("/components.css", componentsCSSHandler)
		r.Mount("/", http.FileServer(pkger.Dir("/frontend/frontserver/static/")))
	})

	return &Mux{r, serverHost, cfg, nil}
}

func (m *Mux) SetErrorRenderer(r ErrorRenderer) {
	m.errR = r
}

func (m *Mux) NewRequest(w http.ResponseWriter, r *http.Request) *Request {
	c, err := client.NewClientFromRequest(m.host, r)
	if err != nil {
		// Host is a constant, so we can panic here.
		log.Panicln("Error making client:", err)
	}

	s := client.NewSessionWithClient(c)

	// Try and grab the username from the cookies. Only use this username value
	// for visual purposes, such as displaying.
	var username string
	if c, err := r.Cookie("username"); err == nil {
		username = c.Value
	}

	return &Request{
		Request: r,
		Writer:  TryFlushWriter(w),
		CommonCtx: CommonCtx{
			Config:   m.cfg,
			Request:  r,
			Session:  s,
			Username: username,
		},
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
			// Copy the status code if available. Else, fallback to 500.
			w.WriteHeader(client.ErrGetStatusCode(err, 500))

			// If there is no error renderer, then we just write the error down
			// in plain text.
			if m.errR == nil {
				fmt.Fprintf(w, "Error: %v", err)
				return
			}

			// Render the error page.
			page, err = m.errR(request, err)
			if err != nil {
				// This shouldn't error out, so we should log it.
				log.Println("Error rendering error page:", err)
				return
			}

		} else {
			// Flush the cookies before writing the body if there is no error.
			request.FlushCookies()
		}

		// Don't render anything if an empty page is returned and there is no
		// error.
		if page == Empty {
			return
		}

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

func (m *Mux) Post(route string, r Renderer) {
	m.Mux.Post(route, m.M(r))
}

func (m *Mux) Delete(route string, r Renderer) {
	m.Mux.Delete(route, m.M(r))
}

// Muxer implements the interface that's passable to pages' mount functions.
type Muxer interface {
	M(Renderer) http.HandlerFunc
}

func (m *Mux) Mount(route string, mounter func(Muxer) http.Handler) {
	m.Mux.Mount(route, mounter(m))
}
