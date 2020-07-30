package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

type ErrUnexpectedStatusCode struct {
	Code   int
	Body   string
	ErrMsg string
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

type Client struct {
	http.Client
	host string
}

// NewClient makes a new client. Host is optional. This client is HTTPS by
// default.
func NewClient(host string) *Client {
	var client = &Client{
		Client: http.Client{
			Timeout: 10 * time.Second,
		},
		host: host,
	}
	if runtime.GOOS != "wasm" {
		client.Client.Jar, _ = cookiejar.New(nil)
	}

	return client
}

func (c *Client) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if c.Jar != nil {
		c.Jar.SetCookies(u, cookies)
	}
}

func (c *Client) SetCookieJar(j http.CookieJar) {
	c.Jar = j
}

func (c *Client) Host() string {
	return c.host
}

// Endpoint returns the HTTPS endpoint, or empty
func (c *Client) Endpoint() string {
	if c.host == "" {
		return "/api/v1"
	}
	if strings.HasPrefix(c.host, "http") {
		return c.host + "/api/v1"
	}
	return fmt.Sprintf("https://%s/api/v1", c.host)
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Support WebAssembly.
	if runtime.GOOS == "wasm" {
		req.Header.Set("js.fetch:credentials", "same-origin")
	}

	r, err := c.Client.Do(req)
	if err != nil {
		return nil, err
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

func (c *Client) DoJSON(req *http.Request, resp interface{}) error {
	q, err := c.Do(req)
	if err != nil {
		return err
	}
	defer q.Body.Close()

	if resp != nil {
		return json.NewDecoder(q.Body).Decode(resp)
	}

	return nil
}

func (c *Client) Post(path string, resp interface{}, v url.Values) error {
	r, err := http.NewRequest("POST", c.Endpoint()+path, strings.NewReader(v.Encode()))
	if err != nil {
		return errors.Wrap(err, "Failed to create request")
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.DoJSON(r, resp)
}

func (c *Client) Get(path string, resp interface{}, v url.Values) error {
	var url = fmt.Sprintf("%s%s?%s", c.Endpoint(), path, v.Encode())

	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create request")
	}

	return c.DoJSON(r, resp)
}
