package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

// StatusCoder is an interface that ErrUnexpectedStatusCode implements.
type StatusCoder interface {
	StatusCode() int
}

// ErrGetStatusCode gets the status code from error, or returns orCode if it
// can't get any.
func ErrGetStatusCode(err error, orCode int) int {
	if scode, ok := err.(StatusCoder); ok {
		return scode.StatusCode()
	}
	return orCode
}

// ErrIs returns true if the given "is" error is wrapped in the "err" error.
func ErrIs(err error, isErrors ...error) bool {
	for _, is := range isErrors {
		if strings.Contains(err.Error(), is.Error()) {
			return true
		}
	}
	return false
}

type ErrUnexpectedStatusCode struct {
	Code   int
	Body   string
	ErrMsg string
}

func (err ErrUnexpectedStatusCode) StatusCode() int {
	return err.Code
}

func (err ErrUnexpectedStatusCode) Error() string {
	var errstr = fmt.Sprintf("Unexpected status code %d", err.Code)
	switch {
	case err.ErrMsg != "":
		errstr += ": " + err.ErrMsg
	case err.Body != "":
		errstr += ", body: " + err.Body
	}

	return errstr
}

// Client contains a single stateful HTTP client. Each session should have its
// own client, as each client has its own cookiejar.
type Client struct {
	http.Client
	ctx    context.Context
	host   *url.URL
	agent  string
	remote string
	socket bool

	// Tries sets the number of tries to connect. Default 4.
	Tries int
}

// NewClient makes a new client. Host is optional. This client is HTTPS by
// default.
func NewClient(host string) (*Client, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse host URL")
	}

	var client = &Client{
		Client: http.Client{
			Jar: NewJar(),
		},
		ctx:  context.Background(),
		host: u,

		Tries: 4,
	}

	return client, nil
}

// dialer to be used w/ unix
var dialer = net.Dialer{
	Timeout: 30 * time.Second,
}

// NewSocketClient makes a new client that dials to the given socket. The given
// host is used for cookies.
func NewSocketClient(host *url.URL, socket string) (*Client, error) {
	var client = &Client{
		Client: http.Client{
			Jar: NewJar(),
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return dialer.DialContext(ctx, "unix", socket)
				},
				DisableKeepAlives: true,
				ForceAttemptHTTP2: true,
			},
		},
		ctx:    context.Background(),
		host:   host,
		socket: true,

		Tries: 4,
	}

	return client, nil
}

// NewSocketClientFromRequest creates a new stateful client with cookies and
// useragents from the request.
func NewHTTPClientFromRequest(backendHTTP string, r *http.Request) (*Client, error) {
	c, err := NewClient(backendHTTP)
	if err != nil {
		return nil, err
	}

	c.ctx = r.Context()
	c.SetCookies(r.Cookies())
	c.SetUserAgent(r.UserAgent())
	c.SetRemoteAddr(r.RemoteAddr)

	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		c.SetRemoteAddr(f)
	}

	return c, nil
}

// NewSocketClientFromRequest creates a new stateful client with cookies and
// useragents from the request.
func NewSocketClientFromRequest(socket string, r *http.Request) (*Client, error) {
	var u = &url.URL{
		Scheme: "http",
		Host:   r.Host,
	}

	if referer, err := url.Parse(r.Header.Get("Referer")); err == nil {
		if referer.Host != "" {
			u.Scheme = referer.Scheme
			u.Host = referer.Host
		}
	}

	c, err := NewSocketClient(u, socket)
	if err != nil {
		return nil, err
	}

	c.ctx = r.Context()
	c.SetUserAgent(r.UserAgent())
	c.SetRemoteAddr(r.RemoteAddr)

	if t, err := r.Cookie("token"); err == nil {
		c.SetCookies([]*http.Cookie{t})
	}

	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		c.SetRemoteAddr(f)
	}

	return c, nil
}

// WithContext shallow-copies the client and returns another one with the
// implicit context set.
func (c *Client) WithContext(ctx context.Context) *Client {
	cpy := new(Client)
	*cpy = *c
	cpy.ctx = ctx
	return cpy
}

// SetRemoteAddr sets the address that will be used for X-Forwarded-For.
func (c *Client) SetRemoteAddr(addr string) {
	c.remote = addr
}

func (c *Client) SetUserAgent(userAgent string) {
	c.agent = userAgent
}

func (c *Client) Cookies() []*http.Cookie {
	return c.Jar.Cookies(c.host)
}

func (c *Client) SetCookies(cookies []*http.Cookie) {
	c.Jar.SetCookies(c.host, cookies)
}

// HostURL returns a copy of the client's host URL with the Path pointing to
// /api/v1.
func (c *Client) HostURL() *url.URL {
	cpy := *c.host
	cpy.Path = "/api/v1"
	return &cpy
}

// Host returns the stringified URL.
func (c *Client) Host() string {
	return c.host.String()
}

// Endpoint returns the HTTPS endpoint, or empty
func (c *Client) Endpoint() string {
	return c.Host() + "/api/v1"
}

func (c *Client) DoOnce(q *http.Request) (*http.Response, error) {
	// Override the UserAgent if we have one.
	if c.agent != "" {
		q.Header.Set("User-Agent", c.agent)
	}

	q.Header.Set("X-Forwarded-For", c.remote)

	// Use HTTP if socket.
	if c.socket {
		q.URL.Scheme = "http"
	}

	r, err := c.Client.Do(q)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		// Start reading the body for the error.
		defer r.Body.Close()

		var unexp = ErrUnexpectedStatusCode{Code: r.StatusCode}

		b, err := ioutil.ReadAll(r.Body)
		if err == nil {
			var errResp smolboard.ErrResponse
			if json.Unmarshal(b, &errResp); errResp.Error != "" {
				unexp.ErrMsg = errResp.Error
			} else {
				if len(b) > 100 {
					unexp.Body = string(b[:97]) + "..."
				} else {
					unexp.Body = string(b)
				}
			}
		}

		return nil, unexp
	}

	return r, nil
}

func (c *Client) Do(req func() (*http.Request, error)) (r *http.Response, err error) {

Retry:
	for i := 0; i < c.Tries; i++ {
		q, err := req()
		if err != nil {
			return nil, err
		}

		// Override the UserAgent if we have one.
		if c.agent != "" {
			q.Header.Set("User-Agent", c.agent)
		}

		q.Header.Set("X-Forwarded-For", c.remote)

		// Use HTTP if socket.
		if c.socket {
			q.URL.Scheme = "http"
		}

		r, err = c.Client.Do(q)
		if err != nil {
			err = errors.Wrap(err, "Failed to send request")
			continue
		}

		switch {
		case r.StatusCode < 200: // 0-199, not sure
			fallthrough
		case r.StatusCode > 499: // > 500, server error
			fallthrough
		case r.StatusCode == http.StatusTooManyRequests: // rate limited
			r.Body.Close()
			continue Retry
		default:
			break Retry // user error, break
		}
	}

	if err == nil && (r.StatusCode < 200 || r.StatusCode > 299) {
		// Start reading the body for the error.
		defer r.Body.Close()

		var unexp = ErrUnexpectedStatusCode{Code: r.StatusCode}

		b, err := ioutil.ReadAll(r.Body)
		if err == nil {
			var errResp smolboard.ErrResponse
			if json.Unmarshal(b, &errResp); errResp.Error != "" {
				unexp.ErrMsg = errResp.Error
			} else {
				if len(b) > 100 {
					unexp.Body = string(b[:97]) + "..."
				} else {
					unexp.Body = string(b)
				}
			}
		}

		return nil, unexp
	}

	return
}

func (c *Client) DoJSON(dst interface{}, q func() (*http.Request, error)) error {
	r, err := c.Do(q)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if dst != nil {
		err := json.NewDecoder(r.Body).Decode(dst)
		return errors.Wrap(err, "Failed to decode JSON")
	}

	return nil
}

func (c *Client) Post(path string, resp interface{}, v url.Values) error {
	return c.Request("POST", path, resp, v)
}

func (c *Client) Get(path string, resp interface{}, v url.Values) error {
	return c.Request("GET", path, resp, v)
}

func (c *Client) Delete(path string, resp interface{}, v url.Values) error {
	return c.Request("DELETE", path, resp, v)
}

func (c *Client) Request(method, path string, resp interface{}, v url.Values) (err error) {
	return c.DoJSON(resp, func() (r *http.Request, err error) {
		switch method {
		case http.MethodPatch, http.MethodPost, http.MethodPut:
			r, err = http.NewRequestWithContext(
				c.ctx,
				method, c.Endpoint()+path, strings.NewReader(v.Encode()),
			)
			if err == nil {
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		default:
			var url = fmt.Sprintf("%s%s?%s", c.Endpoint(), path, v.Encode())
			r, err = http.NewRequestWithContext(c.ctx, method, url, nil)
		}

		return r, errors.Wrap(err, "Failed to create request")
	})
}
