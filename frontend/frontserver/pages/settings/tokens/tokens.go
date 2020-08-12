package tokens

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
		pkger.Include("/frontend/frontserver/pages/settings/tokens/tokens.css"),
	)
}

var tmpl = render.BuildPage("cpanel", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/settings/tokens/tokens.html"),
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
	mux.Post("/", muxer.M(createToken))
	mux.Post("/{token}/delete", muxer.M(deleteToken))
	return mux
}

func renderPage(r *render.Request) (render.Render, error) {
	t, err := r.Session.Tokens()
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

func createToken(r *render.Request) (render.Render, error) {
	u, err := strconv.Atoi(r.FormValue("uses"))
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to parse uses int")
	}

	_, err = r.Session.CreateToken(u)
	if err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}

func deleteToken(r *render.Request) (render.Render, error) {
	if err := r.Session.DeleteToken(chi.URLParam(r.Request, "token")); err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}
