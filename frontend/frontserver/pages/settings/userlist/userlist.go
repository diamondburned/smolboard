package userlist

import (
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
		pkger.Include("/frontend/frontserver/pages/settings/userlist/userlist.css"),
	)
}

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/settings/userlist/userlist.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{},
})

const PageSize = 25

type renderCtx struct {
	render.CommonCtx
	smolboard.UserList
	Page int // ?p=X
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))
	return mux
}

func renderPage(r *render.Request) (render.Render, error) {
	var page = 1
	if str := r.FormValue("p"); str != "" {
		p, err := strconv.Atoi(str)
		if err != nil {
			return render.Empty, errors.Wrap(err, "Failed to parse page")
		}
		page = p
	}

	u, err := r.Session.Users(PageSize, page-1)
	if err != nil {
		return render.Empty, err
	}

	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		UserList:  u,
		Page:      page,
	}

	return render.Render{
		Title: "Users",
		Body:  tmpl.Render(renderCtx),
	}, nil
}
