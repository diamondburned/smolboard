package pager

import (
	"html/template"
	"math"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/search"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/pager/pager.css"),
	)
}

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/pager/pager.html"),
	Components: map[string]render.Component{
		"search": search.Component,
	},
	Functions: template.FuncMap{
		"numPages": func(max int) int {
			return int(math.Ceil(float64(max) / PageSize))
		},

		"dec": func(i int) int { return i - 1 },
		"inc": func(i int) int { return i + 1 },
	},
}
