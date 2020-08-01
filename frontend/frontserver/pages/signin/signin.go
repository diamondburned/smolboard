package signin

import (
	"net/http"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/signin/signin.css"),
	)
}

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/signin/signin.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
})

type renderCtx struct {
	render.CommonCtx
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(pageRender))
	mux.Post("/", muxer.M(post))
	return mux
}

func pageRender(r render.Request) (render.Render, error) {
	return pageRenderErr(r, nil)
}

func pageRenderErr(r render.Request, err error) (render.Render, error) {
	return render.Empty, nil
}

func post(r render.Request) (render.Render, error) {
	s, err := r.Session.Signin(r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		return render.Empty, err
	}

	// Confirm that we do indeed have a token cookie.
	tcookie := r.TokenCookie()
	if tcookie == nil {
		return render.Empty, errors.New("Server error: token cookie not found")
	}

	// Copy the token cookie and use it for the username. This ensures that the
	// expiry dates and other fields are kept the same.
	ucookie := *tcookie
	ucookie.Name = "username"
	ucookie.Value = s.Username

	return render.Empty, nil
}
