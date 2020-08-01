package frontserver

import (
	"net/http"

	"github.com/diamondburned/smolboard/frontend/frontserver/pages/errorpage"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/gallery"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/home"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/post"
	"github.com/diamondburned/smolboard/frontend/frontserver/pages/signin"
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

func New(serverHost string, cfg FrontConfig) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := render.NewMux(serverHost, cfg.Config)
	r.SetErrorRenderer(errorpage.RenderError)
	r.Get("/", home.Render)
	r.Mount("/posts", gallery.Mount)
	r.Mount("/posts/{id}", post.Mount)
	r.Mount("/signin", signin.Mount)

	return r, nil
}
