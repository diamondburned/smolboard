package gallery

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/diamondburned/smolboard/frontend/frontserver/components/footer"
	"github.com/diamondburned/smolboard/frontend/frontserver/components/nav"
	"github.com/diamondburned/smolboard/frontend/frontserver/internal/unblur"
	"github.com/diamondburned/smolboard/frontend/frontserver/render"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
)

func init() {
	render.RegisterCSSFile(
		pkger.Include("/frontend/frontserver/pages/gallery/gallery.css"),
	)
}

const MaxThumbSize = 300
const PageSize = 25

var tmpl = render.BuildPage("home", render.Page{
	Template: pkger.Include("/frontend/frontserver/pages/gallery/gallery.html"),
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"footer": footer.Component,
	},
	Functions: map[string]interface{}{
		"isImage": func(ctype string) bool { return genericMIME(ctype) == "image" },
		"isVideo": func(ctype string) bool { return genericMIME(ctype) == "video" },

		"negInf": func(i int) string {
			if i == -1 {
				return "∞"
			}
			return strconv.Itoa(i)
		},

		"numPages": func(max int) int {
			return int(math.Ceil(float64(max) / PageSize))
		},

		"dec": func(i int) int { return i - 1 },
		"inc": func(i int) int { return i + 1 },
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
	smolboard.SearchResults

	// Tokens is non-nil if IsAdmin returns true.
	smolboard.TokenList

	Query string // ?q=X
	Page  int    // ?p=X
}

func (r renderCtx) AllowedUploadPerms() []smolboard.Permission {
	// Guests can't upload.
	if r.User == nil || r.User.Permission == smolboard.PermissionGuest {
		return nil
	}

	var allPerm = smolboard.AllPermissions()

	// Iterate over all permissions except guest.
	for i, perm := range allPerm[1:] {
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

	return r.Session.PostThumbURL(p)
}

// IsAdmin returns true if the current user is an admin.
func (r renderCtx) IsAdmin() bool {
	return true &&
		r.User != nil &&
		r.User.Username == r.Username &&
		r.User.Permission >= smolboard.PermissionAdministrator
}

func (r renderCtx) MinTokenUses() int {
	if !r.IsAdmin() {
		return 0
	}

	if r.User.Permission == smolboard.PermissionOwner {
		return -1
	}

	return 1
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

	p, err := r.Session.PostSearch(query, PageSize, page-1)
	if err != nil {
		return render.Empty, err
	}

	// Push thumbnails before page load.
	for _, post := range p.Posts {
		r.Push(r.Session.PostThumbURL(post))
	}

	var renderCtx = renderCtx{
		CommonCtx:     r.CommonCtx,
		SearchResults: p,

		Page:  page,
		Query: query,
	}

	if renderCtx.IsAdmin() {
		t, err := r.Session.ListTokens()
		if err != nil {
			return render.Empty, errors.Wrap(err, "Failed to get tokens")
		}
		renderCtx.TokenList = t
	}

	return render.Render{
		Title: "Gallery",
		Body:  tmpl.Render(renderCtx),
	}, nil
}

func uploader(r *render.Request) (render.Render, error) {
	q, err := http.NewRequest("POST", r.Session.Endpoint("/posts"), r.Body)
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
