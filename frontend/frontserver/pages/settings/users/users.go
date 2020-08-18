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
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile("pages/settings/users/users.css")
}

var tmpl = render.BuildPage("home", render.Page{
	Template: "pages/settings/users/users.html",
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
	Me    smolboard.UserPart
	Query string // ?q=X
	Page  int    // ?p=X
}

// AllowedPromotes returns the allowed permissions the current user can promote
// the target user to.
func (r renderCtx) AllowedPromotes(target smolboard.UserPart) []smolboard.Permission {
	// Fast path.
	if r.Me.Permission < target.Permission {
		return nil
	}
	if r.Me.Username == target.Username {
		return nil
	}

	var self = r.Me.Username == target.Username
	var allp = smolboard.AllPermissions()

	// Skip guest and owner.
	allp = allp[1 : len(allp)-1]

	for i, perm := range allp {
		if err := r.Me.Permission.HasPermOverUser(perm, target.Permission, self); err != nil {
			return allp[:i]
		}
	}

	return allp
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))

	mux.Route("/{username}", func(mux chi.Router) {
		mux.Post("/delete", muxer.M(deleteUser))
		mux.Post("/promote", muxer.M(promoteUser))
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
		return render.Empty, errors.Wrap(err, "Failed to get users")
	}

	m, err := r.Me()
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to get self")
	}

	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		UserList:  u,
		Me:        m,
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

func promoteUser(r *render.Request) (render.Render, error) {
	var username = chi.URLParam(r.Request, "username")

	p, err := strconv.Atoi(r.FormValue("p"))
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to parse permission")
	}

	if err := r.Session.SetUserPermission(username, smolboard.Permission(p)); err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}
