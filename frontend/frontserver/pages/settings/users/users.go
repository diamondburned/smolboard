package users

import (
	"net/http"
	"strconv"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/pager"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/settings/users/users.css"),
	)
}

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/settings/users/users.html"),
	Components: map[string]render.Component{
		"pager":  pager.Component,
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{},
})

type renderCtx struct {
	render.CommonCtx
	smolboard.UserList
	Query string // ?q=X
	Page  int    // ?p=X
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))

	mux.Route("/{username}", func(mux chi.Router) {
		mux.Post("/delete", muxer.M(deleteUser))
	})
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

	var query = r.FormValue("q")

	u, err := r.Session.SearchUsers(query, pager.PageSize, page-1)
	if err != nil {
		return render.Empty, err
	}

	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		UserList:  u,
		Page:      page,
		Query:     query,
	}

	return render.Render{
		Title: "Users",
		Body:  tmpl.Render(renderCtx),
	}, nil
}

func deleteUser(r *render.Request) (render.Render, error) {
	if err := r.Session.DeleteUser(chi.URLParam(r.Request, "username")); err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}
