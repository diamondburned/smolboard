package signin

import (
	"net/http"

	"github.com/diamondburned/smolboard/client"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/errbox"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile("pages/signin/signin.css")
}

var tmpl = render.BuildPage("signin", render.Page{
	Template: "pages/signin/signin.html",
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
		Title: "Sign In",
		Body:  tmpl.Render(renderCtx),
	}, nil
}

func handlePOST(r *render.Request) (render.Render, error) {
	var (
		username = r.FormValue("username")
		password = r.FormValue("password")
	)

	_, err := r.Session.Signin(username, password)
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

	r.Redirect("/posts", http.StatusSeeOther)
	return render.Empty, nil
}

func MountSignOut(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Post("/", muxer.M(signout))
	return mux
}

func signout(r *render.Request) (render.Render, error) {
	if err := r.Session.Signout(); err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}
