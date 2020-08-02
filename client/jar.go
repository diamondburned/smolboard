package client

import (
	"net/http"
	"net/url"
	"sync"
)

type Jar struct {
	mu sync.Mutex
	cs map[string][]*http.Cookie
}

// NewJar makes a new cookiejar.
func NewJar() *Jar {
	return &Jar{
		cs: make(map[string][]*http.Cookie, 1),
	}
}

// SetCookies handles the receipt of the cookies in a reply for the
// given URL.  It may or may not choose to save the cookies, depending
// on the jar's policy and implementation.
func (jar *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.mu.Lock()
	jar.cs[u.Host] = cookies
	jar.mu.Unlock()
}

// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265.
func (jar *Jar) Cookies(u *url.URL) []*http.Cookie {
	return jar.cs[u.Host]
}
