package tokenlist

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/settings/tokenlist/tokenlist.css"),
	)
}

var tmpl = render.BuildPage("cpanel", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/settings/tokenlist/tokenlist.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: template.FuncMap{
		"negInf": func(i int) string {
			if i == -1 {
				return "∞"
			}
			return strconv.Itoa(i)
		},
	},
})

type renderCtx struct {
	render.CommonCtx
	smolboard.TokenList
}

func (r renderCtx) MinTokenUses() int {
	if r.SelfPerm == smolboard.PermissionOwner {
		return -1
	}

	return 1
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))
	return mux
}

func renderPage(r *render.Request) (render.Render, error) {
	t, err := r.Session.ListTokens()
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to get tokens")
	}

	return render.Render{
		Title:       "Tokens",
		Description: fmt.Sprintf("%d active tokens.", len(t.Tokens)),
		Body: tmpl.Render(renderCtx{
			CommonCtx: r.CommonCtx,
			TokenList: t,
		}),
	}, nil
}
