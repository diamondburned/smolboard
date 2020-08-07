package client

import (
	"net/http"
	"net/url"
)

// Jar implements a thread-unsafe single-domain cookiejar.
type Jar struct {
	host string
	cook []*http.Cookie
}

// NewJar makes a new cookiejar.
func NewJar() *Jar {
	return &Jar{}
}

// SetCookies handles the receipt of the cookies in a reply for the
// given URL. It may or may not choose to save the cookies, depending
// on the jar's policy and implementation.
func (jar *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.host = u.Host
	jar.cook = cookies
}

// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265.
func (jar *Jar) Cookies(u *url.URL) []*http.Cookie {
	if u.Host != jar.host {
		return nil
	}
	return jar.cook
}
