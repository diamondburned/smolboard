package middleware

import (
	"net/http"
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
