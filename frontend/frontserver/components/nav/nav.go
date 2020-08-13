package nav

import (
	"github.com/diamondburned/smolboard/frontend/frontserver/components/search"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
)

func init() {
	render.RegisterCSSFile("components/nav/nav.css")
}

var Component = render.Component{
	Template: "components/nav/nav.html",
	Components: map[string]render.Component{
		"search": search.Component,
	},
}
