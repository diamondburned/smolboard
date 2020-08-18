package errorpage

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
)

func init() {
	render.RegisterCSSFile("pages/errorpage/errorpage.css")
}

var tmpl = render.BuildPage("home", render.Page{
	Template: "pages/errorpage/errorpage.html",
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
})

type renderCtx struct {
	render.CommonCtx
	Errors [][]string
}

func RenderError(r *render.Request, err error) (render.Render, error) {
	var lines = strings.Split(err.Error(), "\n")
	var errors = make([][]string, len(lines))

	for i, line := range lines {
		var parts = strings.SplitAfter(line, ": ")

		// Capitalize every single error's first letter.
		for i, err := range parts {
			f, sze := utf8.DecodeRune([]byte(err))
			if sze > 0 {
				f = unicode.ToUpper(f)
				parts[i] = string(f) + err[sze:]
			}

			// Append a period at the end for formality.
			if i == len(parts)-1 {
				parts[i] += "."
			}
		}

		errors[i] = parts
	}

	return render.Render{
		Body: tmpl.Render(renderCtx{
			CommonCtx: r.CommonCtx,
			Errors:    errors,
		}),
	}, nil
}
