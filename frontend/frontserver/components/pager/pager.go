package pager

import (
	"html/template"
	"math"
	"strconv"

	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile("components/pager/pager.css")
}

const PageSize = 25

var Component = render.Component{
	Template: "components/pager/pager.html",
	Functions: template.FuncMap{
		"numPages": func(max int) int {
			return int(math.Ceil(float64(max) / PageSize))
		},

		"PageSize": func() int { return PageSize },

		"dec": func(i int) int { return i - 1 },
		"inc": func(i int) int { return i + 1 },
	},
}

// Page returns a 1-indexed page count parsed from "p".
func Page(r *render.Request) (int, error) {
	var page = 1
	if str := r.FormValue("p"); str != "" {
		p, err := strconv.Atoi(str)
		if err != nil {
			return 0, errors.Wrap(err, "Failed to parse page")
		}
		page = p
	}
	return page, nil
}
