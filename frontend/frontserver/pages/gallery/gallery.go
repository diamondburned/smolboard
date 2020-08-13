package gallery

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/pager"
	"github.com/diamondburned/smolboard/frontend/frontserver/internal/unblur"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile("pages/gallery/gallery.css")
}

var tmpl = render.BuildPage("home", render.Page{
	Template: "pages/gallery/gallery.html",
	Components: map[string]render.Component{
		"pager":  pager.Component,
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{
		"isImage": func(ctype string) bool { return genericMIME(ctype) == "image" },
		"isVideo": func(ctype string) bool { return genericMIME(ctype) == "video" },
	},
})

const MaxThumbSize = 300

func genericMIME(mime string) string {
	if parts := strings.Split(mime, "/"); len(parts) > 0 {
		return parts[0]
	}
	return ""
}

type renderCtx struct {
	render.CommonCtx
	smolboard.SearchResults

	Query string   // ?q=X
	Page  int      // ?p=X
	Types []string // MIME types
}

func (r renderCtx) IsMe() bool {
	return r.User != nil && r.User.Username == r.Username
}

func (r renderCtx) AllowedTypes() string {
	return strings.Join(r.Types, ",")
}

func (r renderCtx) AllowedUploadPerms() []smolboard.Permission {
	// Guests can't upload.
	if !r.IsMe() || r.User.Permission == smolboard.PermissionGuest {
		return nil
	}

	var allPerm = smolboard.AllPermissions()

	// Iterate over all permissions.
	for i, perm := range allPerm {
		if perm > r.User.Permission {
			// Slice 0:i to allow guests.
			return allPerm[:i]
		}
	}

	// Highest permission since the larger-than condition is never reached.
	return allPerm
}

func (r renderCtx) SizeAttr(p smolboard.Post) template.HTMLAttr {
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

	return r.Session.PostThumbPath(p)
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(pageRender))
	mux.Post("/", muxer.M(uploader))
	return mux
}

func pageRender(r *render.Request) (render.Render, error) {
	var page = 1
	if str := r.FormValue("p"); str != "" {
		p, err := strconv.Atoi(str)
		if err != nil {
			return render.Empty, errors.Wrap(err, "Failed to parse page")
		}
		page = p
	}

	var query = r.FormValue("q")

	p, err := r.Session.PostSearch(query, pager.PageSize, page-1)
	if err != nil {
		return render.Empty, err
	}

	// Push thumbnails before page load.
	for _, post := range p.Posts {
		r.Push(r.Session.PostThumbPath(post))
	}

	var renderCtx = renderCtx{
		CommonCtx:     r.CommonCtx,
		SearchResults: p,

		Page:  page,
		Query: query,
	}

	// If we can upload, then we should get the supported MIME types for the
	// uploader form.
	if renderCtx.IsMe() {
		m, err := r.Session.AllowedTypes()
		if err != nil {
			return render.Empty, errors.Wrap(err, "Failed to get allowed types")
		}
		renderCtx.Types = m
	}

	return render.Render{
		Title:       "Gallery",
		Description: fmt.Sprintf("%d posts with %s", p.Total, query),
		Body:        tmpl.Render(renderCtx),
	}, nil
}

func uploader(r *render.Request) (render.Render, error) {
	q, err := http.NewRequestWithContext(r.Context(), "POST", r.Session.Endpoint("/posts"), r.Body)
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to create request")
	}

	// Copy all headers, including Content-Type.
	for k, v := range r.Header {
		q.Header[k] = v
	}

	p, err := r.Session.Client.Do(q)
	if err != nil {
		return render.Empty, err
	}
	// We don't need the posts responded.
	p.Body.Close()

	r.Redirect("/posts", http.StatusSeeOther)
	return render.Empty, nil
}

func MountTokenRoutes(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Post("/", muxer.M(createToken))
	mux.Post("/{token}/delete", muxer.M(deleteToken))
	return mux
}

func createToken(r *render.Request) (render.Render, error) {
	u, err := strconv.Atoi(r.FormValue("uses"))
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to parse uses value")
	}

	_, err = r.Session.CreateToken(u)
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to create a token")
	}

	r.Redirect(r.Referer(), http.StatusSeeOther)
	return render.Empty, nil
}

func deleteToken(r *render.Request) (render.Render, error) {
	panic("Implement me")
}
