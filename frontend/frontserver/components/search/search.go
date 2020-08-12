package search

import (
	"html/template"
	"net/http"

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
	Functions: template.FuncMap{
		"searchQ": func(r *http.Request) string {
			if r == nil || r.URL == nil {
				return ""
			}

			if r.URL.Path == "/posts" {
				return r.FormValue("q")
			}

			return ""
		},
	},
}
