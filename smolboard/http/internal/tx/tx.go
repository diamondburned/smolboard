package tx

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/diamondburned/smolboard/smolboard/db"
	"github.com/diamondburned/smolboard/smolboard/http/upload"
	"github.com/diamondburned/smolboard/smolboard/httperr"
	"github.com/go-chi/chi"
)

type Request struct {
	*http.Request
	wr http.ResponseWriter
	Up upload.UploadConfig
	Tx db.Transaction
}

// Param is a helper function that returns a URL parameter from chi.
func (r Request) Param(s string) string {
	return chi.URLParam(r.Request, s)
}

// SetSession sets the written token cookie to the given session. The given
// session can be nil.
func (r *Request) SetSession(s *db.Session) {
	if s != nil {
		r.Tx.Session = *s
	} else {
		r.Tx.Session = db.Session{}
	}

	http.SetCookie(r.wr, &http.Cookie{
		Name:    "token",
		Value:   r.Tx.Session.AuthToken,
		Expires: time.Unix(0, r.Tx.Session.Deadline),
		Secure:  true,
	})
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

	err := m.db.AcquireGuest(r.Context(),
		func(tx *db.Transaction) (err error) {
			v, err = h(Request{r, w, m.up, *tx})
			return
		},
	)

	if err != nil {
		RenderError(w, err)
		return
	}

	render(w, v)
}

func (m Middleware) auth(h Handler, c *http.Cookie, w http.ResponseWriter, r *http.Request) {
	// Check cookies for the session.
	c, err := r.Cookie("token")
	if err != nil {
		RenderWrap(w, err, 403, "Missing token cookie")
		return
	}

	var v interface{}
	var e int64 // new expiry

	err = m.db.Acquire(r.Context(), c.Value,
		func(tx *db.Transaction) (err error) {
			// Update the cookie's expiry date. If this is the guest session,
			// then the expiry would be 0. As such, we'll check later and not
			// set the cookie if so.
			e = tx.Session.Deadline

			// Call the given handler with the transaction.
			v, err = h(Request{r, w, m.up, *tx})

			return
		},
	)

	if err != nil {
		RenderError(w, err)
		return
	}

	// Save the cookie if the expiry time is valid.
	if e > 0 {
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

// ErrResponse is the structure of the response when the request returns an
// error.
type ErrResponse struct {
	Error string `json:"error"`
}

func RenderWrap(w http.ResponseWriter, err error, code int, wrap string) {
	RenderError(w, httperr.Wrap(err, code, wrap))
}

func RenderError(w http.ResponseWriter, err error) {
	code := httperr.ErrCode(err)
	w.WriteHeader(code)

	var jsonError = ErrResponse{
		Error: err.Error(),
	}

	if err := json.NewEncoder(w).Encode(jsonError); err != nil {
		log.Println("Encode failed:", err)
	}
}
