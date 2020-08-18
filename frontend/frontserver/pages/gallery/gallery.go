package gallery

import (
	"encoding/json"
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

	DefaultUploadPerm smolboard.Permission
}

func (r renderCtx) IsMe() bool {
	return r.User != nil && r.User.Username == r.Username
}

func (r renderCtx) AllowedTypes() string {
	return strings.Join(r.Types, ",")
}

func (r renderCtx) AllowedUploadPerms() []smolboard.Permission {
	if !r.IsMe() {
		return nil
	}

	return r.User.AllowedPermissions()
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
	page, err := pager.Page(r)
	if err != nil {
		return render.Empty, err
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

	var defperm = smolboard.PermissionUser
	if c := r.Cookie("uploadperm"); c != nil {
		i, err := strconv.Atoi(c.Value)
		if err == nil {
			if p := smolboard.Permission(i); p.IsValid() {
				defperm = p
			}
		}
	}

	var renderCtx = renderCtx{
		CommonCtx:     r.CommonCtx,
		SearchResults: p,

		Page:  page,
		Query: query,

		DefaultUploadPerm: defperm,
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

	p, err := r.Session.Client.DoOnce(q)
	if err != nil {
		return render.Empty, err
	}
	defer p.Body.Close()

	// Shitty hack.
	var posts []smolboard.Post

	if err := json.NewDecoder(p.Body).Decode(&posts); err != nil {
		return render.Empty, errors.Wrap(err, "Invalid JSON response from server")
	}

	if len(posts) > 0 {
		// Store the uploaded post's permission as a cookie for future
		// preference.
		r.SetWeakCookie("uploadperm", posts[0].Permission.StringInt())
	}

	r.Redirect("/posts", http.StatusSeeOther)
	return render.Empty, nil
}
