package pager

import (
	"html/template"
	"math"

	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/components/pager/pager.css"),
	)
}

const PageSize = 25

var Component = render.Component{
	Template: pkger.Include("/frontend/frontserver/components/pager/pager.html"),
	Functions: template.FuncMap{
		"numPages": func(max int) int {
			return int(math.Ceil(float64(max) / PageSize))
		},

		"PageSize": func() int { return PageSize },

		"dec": func(i int) int { return i - 1 },
		"inc": func(i int) int { return i + 1 },
	},
}
