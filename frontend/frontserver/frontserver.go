package frontserver

import (
	"net/http"

	"github.com/diamondburned/smolboard/frontend/frontserver/pages/errorpage"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/gallery"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/home"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/post"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/settings"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/signin"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/signup"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
)

type FrontConfig struct {
	render.Config
}

func NewConfig() FrontConfig {
	return FrontConfig{
		Config: render.NewConfig(),
	}
}

func (c *FrontConfig) Validate() error {
	return nil
}

func New(socket string, cfg FrontConfig) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := render.NewMux(socket, cfg.Config)
	bind(r)

	return r, nil
}

func NewWithHTTPBackend(endpoint string, cfg FrontConfig) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := render.NewHTTPMux(endpoint, cfg.Config)
	bind(r)

	return r, nil
}

func bind(r *render.Mux) {
	r.SetErrorRenderer(errorpage.RenderError)
	r.Get("/", home.Render)
	r.Mount("/posts", gallery.Mount)
	r.Mount("/posts/{id}", post.Mount)
	r.Mount("/tokens", gallery.MountTokenRoutes)
	r.Mount("/signin", signin.Mount)
	r.Mount("/signup", signup.Mount)
	r.Mount("/signout", signin.MountSignOut)
	r.Mount("/settings", settings.Mount)
	// r.Mount("/user-settings", userlist.Mount)
	// r.Mount("/token-settings", tokenlist.Mount)
}
