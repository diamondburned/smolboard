package search

import (
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/search/search.css"),
	)
}

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/search/search.html"),
}
