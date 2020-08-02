package errorpage

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/errorpage/errorpage.css"),
	)
}

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/errorpage/errorpage.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
})

type renderCtx struct {
	render.CommonCtx
	Errors []string
}

func RenderError(r *render.Request, err error) (render.Render, error) {
	var errors = strings.SplitAfter(err.Error(), ": ")

	// Capitalize every single error's first letter.
	for i, err := range errors {
		f, sze := utf8.DecodeRune([]byte(err))
		if sze > 0 {
			f = unicode.ToUpper(f)
			errors[i] = string(f) + err[sze:]
		}

		// Append a period at the end for formality.
		if i == len(errors)-1 {
			errors[i] += "."
		}
	}

	return render.Render{
		Body: tmpl.Render(renderCtx{
			CommonCtx: r.CommonCtx,
			Errors:    errors,
		}),
	}, nil
}
