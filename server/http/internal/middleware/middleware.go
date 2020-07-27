package middleware

import (
	"bufio"
	"io"
	"net/http"

	"github.com/c2h5oh/datasize"
	"github.com/diamondburned/smolboard/server/http/upload"
	"github.com/go-chi/chi/middleware"
)

var (
	Compress  = middleware.Compress
	RealIP    = middleware.RealIP
	Throttle  = middleware.Throttle
	Recoverer = middleware.Recoverer
)

// F represents a middleware function.
type F = func(http.Handler) http.Handler

// H represents a short middleware function signature that returns a boolean. If
// this boolean is false, then the middleware chain is broken.
type H = func(w http.ResponseWriter, r *http.Request) bool

// P wraps the given middleware handler to be called as a prefix to the next
// handler in chain.
func P(h H) F {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !h(w, r) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func LimitBody(size datasize.ByteSize) F {
	return P(func(w http.ResponseWriter, r *http.Request) bool {
		r.Body = newReadCloser(
			upload.NewLimitedReader(
				bufio.NewReader(r.Body),
				int64(size.Bytes()),
			),
			r.Body,
		)
		return true
	})
}

type readCloser struct {
	io.Reader
	io.Closer
}

func newReadCloser(r io.Reader, c io.Closer) readCloser {
	return readCloser{r, c}
}
