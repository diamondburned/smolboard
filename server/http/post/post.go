package post

import (
	"net/http"
	"strconv"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/http/internal/form"
	"github.com/diamondburned/smolboard/server/http/internal/limit"
	"github.com/diamondburned/smolboard/server/http/internal/tx"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
)

func Mount(m tx.Middlewarer) http.Handler {
	mux := chi.NewMux()
	mux.Use(limit.RateLimit(64))
	mux.Get("/", m(ListPosts))
	// POST but parse form before entering a transaction.
	mux.With(preparseMultipart, limit.RateLimit(2)).Post("/", m(UploadPost))

	mux.Route("/{id}", func(r chi.Router) {
		// GET gives both tags and permission.
		r.Get("/", m(GetPost))
		r.Delete("/", m(DeletePost))

		r.Patch("/permission", m(SetPostPermission))

		r.Route("/tags", func(r chi.Router) {
			r.Put("/", m(TagPost))
			r.Post("/", m(TagPost))
			r.Delete("/", m(UntagPost))
		})
	})

	return mux
}

func preparseMultipart(next http.Handler) http.Handler {
	const formLimit = int64(2 * datasize.MB)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(formLimit); err != nil {
			tx.RenderWrap(w, err, 400, "Failed to parse form")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ListParams is the URL parameter for post listing pagination.
type ListParams struct {
	Query string `schema:"q"`
	Count uint   `schema:"c"`
	Page  uint   `schema:"p"`
}

func ListPosts(r tx.Request) (interface{}, error) {
	var params = ListParams{Count: 24}

	if err := form.Unmarshal(r, &params); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return r.Tx.PostSearch(params.Query, params.Count, params.Page)
}

func GetPost(r tx.Request) (interface{}, error) {
	i, err := strconv.ParseInt(r.Param("id"), 10, 64)
	if err != nil {
		return nil, smolboard.ErrPostNotFound
	}

	return r.Tx.Post(i)
}

type UploadParams struct {
	Permission smolboard.Permission `schema:"p"` // default Normal
}

func UploadPost(r tx.Request) (interface{}, error) {
	var p UploadParams

	if err := form.Unmarshal(r, &p); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	files, ok := r.MultipartForm.File["file"]
	if !ok {
		return nil, httperr.New(400, "missing field 'file' in form")
	}

	posts, err := r.Up.CreatePosts(files)
	if err != nil {
		return nil, err
	}

	for _, post := range posts {
		// Set the post's permission.
		post.Permission = p.Permission

		if err := r.Tx.SavePost(post); err != nil {
			// Something failed. Before we exit, we need to clean up all
			// downloaded files.
			r.Up.RemovePosts(posts)

			return nil, errors.Wrap(err, "Failed to save post")
		}
	}

	return posts, nil
}

func DeletePost(r tx.Request) (interface{}, error) {
	i, err := strconv.ParseInt(r.Param("id"), 10, 64)
	if err != nil {
		return nil, smolboard.ErrPostNotFound
	}

	return nil, r.Tx.DeletePost(i)
}

type PostPermission struct {
	Permission smolboard.Permission `schema:"p,required"`
}

// SetPostPermission: /{id}?p=0
func SetPostPermission(r tx.Request) (interface{}, error) {
	i, err := strconv.ParseInt(r.Param("id"), 10, 64)
	if err != nil {
		return nil, smolboard.ErrPostNotFound
	}

	var p PostPermission

	if err := form.Unmarshal(r, &p); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return nil, r.Tx.SetPostPermission(i, p.Permission)
}

type Tag struct {
	Tag string `schema:"t,required"`
}

func TagPost(r tx.Request) (interface{}, error) {
	i, err := strconv.ParseInt(r.Param("id"), 10, 64)
	if err != nil {
		return nil, smolboard.ErrPostNotFound
	}

	var t Tag

	if err := form.Unmarshal(r, &t); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return nil, r.Tx.TagPost(i, t.Tag)
}

func UntagPost(r tx.Request) (interface{}, error) {
	i, err := strconv.ParseInt(r.Param("id"), 10, 64)
	if err != nil {
		return nil, smolboard.ErrPostNotFound
	}

	var t Tag

	if err := form.Unmarshal(r, &t); err != nil {
		return nil, httperr.Wrap(err, 400, "Invalid form")
	}

	return nil, r.Tx.UntagPost(i, t.Tag)
}
