package posts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/diamondburned/smolboard/client"
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
	render.RegisterCSSFile("pages/settings/posts/posts.css")
}

var tmpl = render.BuildPage("posts-settings", render.Page{
	Template: "pages/settings/posts/posts.html",
	Components: map[string]render.Component{
		"nav":    nav.Component,
		"pager":  pager.Component,
		"footer": footer.Component,
	},
	Functions: template.FuncMap{
		"tagRemoveName": func(tag string) string {
			if strings.HasPrefix(tag, "-") {
				return "untag"
			}
			return ""
		},

		"tagEscape": smolboard.EscapeTag,
	},
})

type renderCtx struct {
	render.CommonCtx
	smolboard.SearchResults
	State State
	Query string
	Page  int
	Me    smolboard.UserPart
}

const MaxThumbSize = 320

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

func (r renderCtx) CanChangePost(p smolboard.Post) bool {
	return r.Me.CanChangePost(p) == nil
}

func Mount(muxer render.Muxer) http.Handler {
	mux := chi.NewMux()
	mux.Get("/", muxer.M(renderPage))
	mux.Post("/reset", muxer.M(resetPOST))
	mux.Post("/apply", muxer.M(processPOST))
	mux.Post("/delete", muxer.M(deletePostsPOST))
	return mux
}

var CookieName = "posts-settings"

type State struct {
	Permission smolboard.Permission `json:"permission"` // -1 == skip
	Selections Int64Set             `json:"selections"`
	Tags       StringSet            `json:"tags"`
}

func NewState() State {
	return State{
		Permission: -1,
		Selections: make(Int64Set),
		Tags:       make(StringSet),
	}
}

func UnmarshalState(r *render.Request) (State, error) {
	var s = NewState()

	// Parse forms.
	if err := r.ParseForm(); err != nil {
		return s, errors.Wrap(err, "Failed to parse form")
	}

	if b64 := r.CookieValue(CookieName); b64 != "" {
		b, err := base64.URLEncoding.DecodeString(b64)
		if err != nil {
			return s, errors.Wrap(err, "Failed to decode base64")
		}

		if err := json.Unmarshal(b, &s); err != nil {
			return s, errors.Wrap(err, "Failed to unmarshal JSON")
		}
	}

	for _, str := range r.Form[FormID] {
		i, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return s, errors.Wrapf(err, "Failed to parse ID %q", str)
		}
		s.Selections.Set(i)
	}

	if permstr := r.FormValue(FormPermission); permstr != "" {
		i, err := strconv.Atoi(permstr)
		if err != nil {
			return s, errors.Wrapf(err, "Failed to parse permission %q", permstr)
		}
		p := smolboard.Permission(i)
		// Ignore -1 as it's a valid value.
		if p != -1 && !p.IsValid() {
			return s, smolboard.ErrInvalidPermission
		}
		s.Permission = p
	}

	for _, str := range r.Form[FormTag] {
		if str == "" {
			continue
		}

		for _, tagstr := range strings.Split(str, ";") {
			// Trim surrounding spaces.
			tagstr = strings.TrimSpace(tagstr)

			// Strip the - prefix for validation.
			var tag = tagstr
			if strings.HasPrefix(tag, "-") {
				tag = tag[1:]
			}

			if err := smolboard.TagIsValid(tag); err != nil {
				return s, errors.Wrap(err, "Failed to validate tag")
			}

			// Append the unmodified version.
			s.Tags.Set(tagstr)
		}
	}

	for _, str := range r.Form[FormUntag] {
		delete(s.Tags, str)
	}

	return s, nil
}

func (s State) MarshalCookie() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", errors.Wrap(err, "")
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

func (s State) PostIsSelected(p smolboard.Post) bool {
	_, ok := s.Selections[p.ID]
	return ok
}

const (
	FormID         = "id"
	FormTag        = "tag"
	FormUntag      = "untag"
	FormPermission = "permission"
)

var allFormKeys = []string{FormID, FormTag, FormUntag, FormPermission}

func renderPage(r *render.Request) (render.Render, error) {
	s, err := UnmarshalState(r)
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to unmarshal settings state")
	}

	b, err := s.MarshalCookie()
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to marshal settings state")
	}

	r.SetWeakCookie(CookieName, b)

	var oldLen = len(r.Form)
	for _, key := range allFormKeys {
		delete(r.Form, key)
	}

	// Attempt to sanitize forms.
	sanitizeForm(r.Form)

	if oldLen != len(r.Form) {
		r.Redirect(fmt.Sprintf("/settings/posts?%s", r.Form.Encode()), http.StatusSeeOther)
		return render.Empty, nil
	}

	u, err := r.Me()
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to get current user")
	}

	page, err := pager.Page(r)
	if err != nil {
		return render.Empty, err
	}

	var query = r.FormValue("q")

	p, err := r.Session.PostSearch(query, pager.PageSize, page-1)
	if err != nil {
		return render.Empty, err
	}

	return render.Render{
		Title: "Posts Settings",
		Body: tmpl.Render(renderCtx{
			CommonCtx:     r.CommonCtx,
			SearchResults: p,
			State:         s,
			Query:         query,
			Page:          page,
			Me:            u,
		}),
	}, nil
}

// sanitizeForm sanitizes the form so that the non-empty values always take
// precedence. This is done to help with multiple form inputs of the same name.
func sanitizeForm(form url.Values) {
	for k, v := range form {
		var f = v[:1]

		for _, v := range v {
			if v != "" {
				f[0] = v
			}
		}

		form[k] = f
	}
}

type JobError struct {
	PostID  int64
	Message string
	Err     error
}

func (err JobError) Error() string {
	return fmt.Sprintf(
		"Failed to process post ID %d: %s: %v",
		err.PostID, err.Message, err.Err,
	)
}

type errStack struct {
	errors []error
}

var (
	_ error              = (*errStack)(nil)
	_ client.StatusCoder = (*errStack)(nil)
)

func newErrStack() *errStack {
	return &errStack{}
}

func (s *errStack) Nil() bool {
	return len(s.errors) == 0
}

func (s *errStack) Error() string {
	var strs = make([]string, len(s.errors))
	for i, err := range s.errors {
		strs[i] = err.Error()
	}

	return strings.Join(strs, "\n")
}

func (s *errStack) StatusCode() int {
	if code := client.ErrGetStatusCode(s.errors[0], 0); code > 0 {
		return code
	}
	return 400
}

func (s *errStack) Add(err error) {
	s.errors = append(s.errors, err)
}

func (s *errStack) JobError(id int64, err error, msg string) {
	s.Add(JobError{
		PostID:  id,
		Message: msg,
		Err:     err,
	})
}

func (s *errStack) JobErrorf(id int64, err error, msgf string, msgv ...interface{}) {
	s.JobError(id, err, fmt.Sprintf(msgf, msgv...))
}

func resetSettings(r *render.Request) error {
	r.SetWeakCookie(CookieName, "")

	u, err := url.Parse(r.Referer())
	if err != nil {
		return errors.Wrap(err, "Failed to parse referer")
	}

	u.RawQuery = "" // wipe form
	r.Redirect(u.String(), http.StatusSeeOther)

	return nil
}

func resetPOST(r *render.Request) (render.Render, error) {
	return render.Empty, resetSettings(r)
}

func processPOST(r *render.Request) (render.Render, error) {
	s, err := UnmarshalState(r)
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to unmarshal settings state")
	}

	if len(s.Selections) == 0 {
		// Nothing to do; go away.
		return render.Empty, resetSettings(r)
	}

	var errors = newErrStack()

	if s.Permission > -1 {
		for postID := range s.Selections {
			if err := r.Session.SetPostPermission(postID, s.Permission); err != nil {
				errors.JobErrorf(postID, err, "Failed to set post ID %d's permission", postID)
			}
		}
	}

	for tag := range s.Tags {
		var action = r.Session.TagPost
		if strings.HasPrefix(tag, "-") {
			tag = tag[1:]
			action = r.Session.UntagPost
		}

		for postID := range s.Selections {
			if err := action(postID, tag); err != nil {
				// Ignore these errors.
				if client.ErrIs(err, smolboard.ErrTagAlreadyAdded, smolboard.ErrPostNotFound) {
					continue
				}

				errors.JobErrorf(postID, err, "Failed to set post ID %d's tag", postID)
			}
		}
	}

	if !errors.Nil() {
		return render.Empty, errors
	}

	return render.Empty, resetSettings(r)
}

func deletePostsPOST(r *render.Request) (render.Render, error) {
	s, err := UnmarshalState(r)
	if err != nil {
		return render.Empty, errors.Wrap(err, "Failed to unmarshal settings state")
	}

	if len(s.Selections) == 0 {
		// Nothing to do; go away.
		return render.Empty, resetSettings(r)
	}

	var errors = newErrStack()

	for postID := range s.Selections {
		if err := r.Session.DeletePost(postID); err != nil {
			errors.JobErrorf(postID, err, "Failed to delete post ID %q", postID)
		}
	}

	if !errors.Nil() {
		return render.Empty, errors
	}

	return render.Empty, resetSettings(r)
}
