package search

import (
	"html/template"
	"net/http"

	"github.com/diamondburned/smolboard/frontend/frontserver/render"
)

func init() {
	render.RegisterCSSFile("components/search/search.css")
}

var Component = render.Component{
	Template: "components/search/search.html",
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
