package post

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/post/post.css"),
	)
}

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/post/post.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{
		"isImage": func(ctype string) bool { return genericMIME(ctype) == "image" },
		"isVideo": func(ctype string) bool { return genericMIME(ctype) == "video" },

		// fmtTime formats the given time to input date value's format.
		"fmtTime": func(t time.Time) string {
			return t.Format("2006-01-02T15:04")
		},

		"allPermissions": func() []smolboard.Permission {
			return smolboard.AllPermissions()
		},
	},
})

func genericMIME(mime string) string {
	if parts := strings.Split(mime, "/"); len(parts) > 0 {
		return parts[0]
	}
	return ""
}

type renderCtx struct {
	render.CommonCtx
	User   smolboard.UserPart
	Post   smolboard.PostExtended
	Poster string
}

func (r renderCtx) CanChangePost() bool {
	return r.User.CanChangePost(r.Post.Post) == nil
}

func (r renderCtx) CanSetPerm(p smolboard.Permission) bool {
	return r.User.CanSetPostPermission(r.Post, p) == nil
}

func (r renderCtx) ImageSizeAttr(p smolboard.Post) template.HTMLAttr {
	if p.Attributes.Height == 0 || p.Attributes.Width == 0 {
		return ""
	}

	return template.HTMLAttr(fmt.Sprintf(
		`width="%d" height="%d"`,
		p.Attributes.Width, p.Attributes.Height,
	))
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(pageRender))
	mux.Post("/tag", muxer.M(tagPost))
	return mux
}

func pageRender(r *render.Request) (render.Render, error) {
	i, err := r.IDParam()
	if err != nil {
		return render.Empty, err
	}

	log.Println("RequestURI:", r.RequestURI)

	p, err := r.Session.Post(i)
	if err != nil {
		return render.Empty, err
	}

	// Try and get the current user, but create a dummy user if we can't.
	u, err := r.Session.Me()
	if err != nil {
		u = smolboard.UserPart{
			Username: r.Username,
		}
	} else {
		// Override the username in the common context so components will use
		// this newly fetched username.
		r.Username = u.Username
	}

	var poster = "Deleted User"
	if p.Poster != nil {
		poster = *p.Poster
	}

	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		User:      u,
		Post:      p,
		Poster:    poster,
	}

	description := strings.Builder{}
	description.Grow(128)

	for _, tag := range p.Tags {
		if description.WriteString(tag.TagName); description.Len() > 128 {
			break
		}
	}

	return render.Render{
		Title:       poster,
		Description: ellipsize(description.String()),
		ImageURL:    r.Session.PostDirectURL(p.Post),
		Body:        tmpl.Render(renderCtx),
	}, nil
}

func tagPost(r *render.Request) (render.Render, error) {
	i, err := r.IDParam()
	if err != nil {
		return render.Empty, err
	}

	if err := r.Session.TagPost(i, r.FormValue("tag")); err != nil {
		return render.Empty, err
	}

	// trim the /tags suffix
	var postURL = path.Dir(r.URL.Path)

	http.Redirect(r.Writer, r.Request, postURL, http.StatusSeeOther)
	return render.Empty, nil
}

func ellipsize(str string) string {
	if len(str) < 128 {
		return str
	}

	return str[:125] + "..."
}
