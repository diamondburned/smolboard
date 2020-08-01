package post

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

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
	Post smolboard.PostWithTags
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

func pageRender(r render.Request) (render.Render, error) {
	i, err := r.IDParam()
	if err != nil {
		return render.Empty, err
	}

	p, err := r.Session.Post(i)
	if err != nil {
		return render.Empty, err
	}

	var renderCtx = renderCtx{
		CommonCtx: r.CommonCtx,
		Post:      p,
	}

	var author = "Deleted User"
	if p.Poster != nil {
		author = *p.Poster
	}

	description := strings.Builder{}
	description.Grow(128)

	for _, tag := range p.Tags {
		if description.WriteString(tag.TagName); description.Len() > 128 {
			break
		}
	}

	return render.Render{
		Title:       author,
		Description: ellipsize(description.String()),
		ImageURL:    r.Session.PostDirectURL(p.Post),
		Body:        tmpl.Render(renderCtx),
	}, nil
}

func tagPost(r render.Request) (render.Render, error) {
	i, err := r.IDParam()
	if err != nil {
		return render.Empty, err
	}

	if err := r.Session.TagPost(i, r.FormValue("tag")); err != nil {
		return render.Empty, err
	}

	// trim the /tags suffix
	var postURL = path.Dir(r.URL.Path)

	http.Redirect(r.Writer, r.Request, postURL, http.StatusTemporaryRedirect)
	return render.Empty, nil
}

func ellipsize(str string) string {
	if len(str) < 128 {
		return str
	}

	return str[:125] + "..."
}
