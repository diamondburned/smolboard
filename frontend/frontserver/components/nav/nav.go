package nav

import (
	"github.com/diamondburned/smolboard/frontend/frontserver/components/search"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/nav/nav.css"),
	)
}

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/nav/nav.html"),
	Components: map[string]render.Component{
		"search": search.Component,
	},
}
