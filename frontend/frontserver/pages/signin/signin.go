package signin

import (
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
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
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{
		"minifyError": minifyError,
	},
})

type renderCtx struct {
	render.CommonCtx
	Error error
}

func minifyError(err error) string {
	var errmsg = err.Error()
	var parts = strings.Split(errmsg, ": ")
	if len(parts) == 0 {
		return ""
	}

	var part = parts[len(parts)-1]
	// Capitalize the first letter.
	f, sz := utf8.DecodeRune([]byte(part))
	if sz > 0 {
		f = unicode.ToUpper(f)
		part = string(f) + part[sz:]
	}

	return part + "."
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(pageRender))
	mux.Post("/", muxer.M(handlePOST))
	mux.Delete("/", muxer.M(handleDELETE))
	return mux
}

func pageRender(r *render.Request) (render.Render, error) {
	return pageRenderErr(r, nil)
}

func pageRenderErr(r *render.Request, err error) (render.Render, error) {
	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		Error:     err,
	}

	return render.Render{
		Title: "Sign in",
		Body:  tmpl.Render(renderCtx),
	}, nil
}

func handlePOST(r *render.Request) (render.Render, error) {
	var (
		username = r.FormValue("username")
		password = r.FormValue("password")
	)

	s, err := r.Session.Signin(username, password)
	if err != nil {
		return render.Empty, err
	}

	log.Println("Received session:", s)

	// Confirm that we do indeed have a token cookie.
	tcookie := r.TokenCookie()
	if tcookie == nil {
		return render.Empty, errors.New("Server error: token cookie not found")
	}

	// Copy the token cookie and use it for the username. This ensures that the
	// expiry dates and other fields are kept the same.
	ucookie := *tcookie
	ucookie.Name = "username"
	ucookie.Value = username

	http.SetCookie(r.Writer, &ucookie)

	r.Redirect(r.Referer(), http.StatusFound)
	return render.Empty, nil
}

func handleDELETE(r *render.Request) (render.Render, error) {
	if err := r.Session.Signout(); err != nil {
		return render.Empty, err
	}

	r.Redirect(r.Referer(), http.StatusFound)
	return render.Empty, nil
}
