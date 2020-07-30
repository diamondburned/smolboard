package gallery

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/diamondburned/smolboard/client"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/search"
	"github.com/diamondburned/smolboard/frontend/frontserver/internal/unblur"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/markbates/pkger"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/gallery/gallery.css"),
	)
}

const MaxThumbSize = 200

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/gallery/gallery.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"search": search.Component,
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
	Posts   []smolboard.Post
	Session *client.Session
	Config  render.Config
}

func (r renderCtx) ImageSizeAttr(p smolboard.Post) template.HTMLAttr {
	if p.Attributes.Height == 0 || p.Attributes.Width == 0 {
		return ""
	}

	w, h := unblur.MaxSize(
		p.Attributes.Width, p.Attributes.Height,
		MaxThumbSize, MaxThumbSize,
	)

	return template.HTMLAttr(fmt.Sprintf(`width="%d" height="%d"`, w, h))
}

func (r renderCtx) InlineImage(p smolboard.Post) interface{} {
	h, err := unblur.InlinePost(p)
	if err == nil {
		return template.URL(h)
	}

	return r.Session.PostThumbURL(p)
}

func Render(r render.Request) (render.Render, error) {
	p, err := r.Session.Posts(25, 0)
	if err != nil {
		return render.Render{}, err
	}

	// Push thumbnails before page load.
	for _, post := range p.Posts {
		r.Push(r.Session.PostThumbURL(post))
	}

	var renderCtx = renderCtx{
		Posts:   p.Posts,
		Session: r.Session,
		Config:  r.Config,
	}

	return render.Render{
		Body: tmpl.Render(renderCtx),
	}, nil
}
