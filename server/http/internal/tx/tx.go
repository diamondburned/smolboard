package tx

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/diamondburned/smolboard/server/db"
	"github.com/diamondburned/smolboard/server/http/upload"
	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/diamondburned/smolboard/smolboard"
	"github.com/go-chi/chi"
)

type Request struct {
	*http.Request
	wr http.ResponseWriter

	Up *upload.UploadConfig
	Tx *db.Transaction
}

// Param is a helper function that returns a URL parameter from chi.
func (r Request) Param(s string) string {
	return chi.URLParam(r.Request, s)
}

// SetSession sets the written token cookie to the given session. The given
// session can be nil.
func (r *Request) SetSession(s *smolboard.Session) {
	log.Println("Setting session to", s)

	if s != nil {
		r.Tx.Session = *s
	} else {
		r.Tx.Session = smolboard.Session{}
	}
}

// Handler is the function signature for transaction handlers. Render could be
// Renderer.
type Handler = func(Request) (render interface{}, err error)

// Renderer is a possible return type for Handler's render.
type Renderer = func(w http.ResponseWriter) error

// Middlewarer is the interface for the transaction middleware.
type Middlewarer = func(Handler) http.HandlerFunc

type Middleware struct {
	db *db.Database
	up upload.UploadConfig
}

var _ Middlewarer = (Middleware{}).M

func NewMiddleware(db *db.Database, up upload.UploadConfig) Middleware {
	return Middleware{db: db, up: up}
}

func (m Middleware) M(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check cookies for the session.
		if c, err := r.Cookie("token"); err == nil {
			m.auth(h, c, w, r)
		} else {
			m.noAuth(h, w, r)
		}
	}
}

func (m Middleware) noAuth(h Handler, w http.ResponseWriter, r *http.Request) {
	var v interface{}
	var s smolboard.Session

	err := m.db.AcquireGuest(r.Context(),
		func(tx *db.Transaction) (err error) {
			v, err = h(Request{r, w, &m.up, tx})
			s = tx.Session
			return
		},
	)

	if err != nil {
		RenderError(w, err)
		return
	}

	// If we have a new session, then send it over.
	if !s.IsZero() {
		// Trim the port if needed.
		var host = r.Host
		// Trick the URL parser into thinking this is a valid URL by prepending a
		// valid scheme.
		if u, err := url.Parse("https://" + host); err == nil {
			host = u.Hostname()
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    s.AuthToken,
			Path:     "/",
			Domain:   host,
			Expires:  time.Unix(0, s.Deadline),
			SameSite: http.SameSiteStrictMode,
		})
	}

	render(w, v)
}

func (m Middleware) auth(h Handler, c *http.Cookie, w http.ResponseWriter, r *http.Request) {
	var v interface{}
	var s smolboard.Session

	err := m.db.Acquire(r.Context(), c.Value,
		func(tx *db.Transaction) (err error) {
			// Call the given handler with the transaction.
			v, err = h(Request{r, w, &m.up, tx})
			s = tx.Session
			return
		},
	)

	if err != nil {
		RenderError(w, err)
		return
	}

	// If the cookie has been changed, then override the cookie's fields to
	// default and send it over.
	if c.Expires.UnixNano() != s.Deadline || c.Value != s.AuthToken {
		c.Path = "/"
		c.Value = s.AuthToken
		c.Expires = time.Unix(0, s.Deadline)
		c.SameSite = http.SameSiteStrictMode
		http.SetCookie(w, c)
	}

	render(w, v)
}

func render(w http.ResponseWriter, v interface{}) {
	if v == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if fn, ok := v.(Renderer); ok {
		if err := fn(w); err != nil {
			RenderError(w, err)
		}
		return
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")

	// Render the body as JSON.
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("Encode failed:", err)
	}
}

func RenderWrap(w http.ResponseWriter, err error, code int, wrap string) {
	RenderError(w, httperr.Wrap(err, code, wrap))
}

func RenderError(w http.ResponseWriter, err error) {
	code := httperr.ErrCode(err)
	w.WriteHeader(code)

	var jsonError = smolboard.ErrResponse{
		Error: err.Error(),
	}

	if err := json.NewEncoder(w).Encode(jsonError); err != nil {
		log.Println("Encode failed:", err)
	}
}
