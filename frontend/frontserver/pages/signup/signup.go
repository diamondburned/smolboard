package signup

import (
	"net/http"

	"github.com/diamondburned/smolboard/client"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/errbox"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/signup/signup.css"),
	)
}

var tmpl = render.BuildPage("signup", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/signup/signup.html"),
	Components: map[string]render.Component{
		"footer": footer.Component,
		"errbox": errbox.Component,
	},
})

type renderCtx struct {
	render.CommonCtx
	Error    error
	Username string
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(pageRender))
	mux.Post("/", muxer.M(handlePOST))
	return mux
}

func pageRender(r *render.Request) (render.Render, error) {
	return pageRenderErr(r, "", nil)
}

func pageRenderErr(r *render.Request, username string, err error) (render.Render, error) {
	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		Error:     err,
		Username:  username,
	}

	return render.Render{
		Title: "Sign Up",
		Body:  tmpl.Render(renderCtx),
	}, nil
}

func handlePOST(r *render.Request) (render.Render, error) {
	var (
		username = r.FormValue("username")
		password = r.FormValue("password")
		token    = r.FormValue("token")
	)

	_, err := r.Session.Signup(username, password, token)
	if err != nil {
		r.Writer.WriteHeader(client.ErrGetStatusCode(err, 500))
		return pageRenderErr(r, username, err)
	}

	// Confirm that we do indeed have a token cookie.
	tcookie := r.TokenCookie()
	if tcookie == nil {
		return render.Empty, errors.New("Server error: token cookie not found")
	}

	// Set the username cookie.
	http.SetCookie(r.Writer, &http.Cookie{
		Name:   "username",
		Value:  username,
		Domain: tcookie.Domain,
	})

	r.Redirect(r.Referer(), http.StatusFound)
	return render.Empty, nil
}
