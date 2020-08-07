package client

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/diamondburned/smolboard/smolboard"
)

type Session struct {
	Client *Client
}

// NewSession creates a new session with the given endpoint. It defaults to the
// global variable if the given endpoint is empty.
func NewSession(host string) (*Session, error) {
	c, err := NewClient(host)
	if err != nil {
		return nil, err
	}

	return NewSessionWithClient(c), nil
}

// NewSessionWithClient creates a new session with a client. Refer to
// NewSession.
func NewSessionWithClient(c *Client) *Session {
	return &Session{
		Client: c,
	}
}

func (s *Session) Endpoint(path string) string {
	return s.Client.Endpoint() + path
}

func (s *Session) Signin(username, password string) (sm smolboard.Session, err error) {
	return sm, s.Client.Post("/signin", &sm, url.Values{
		"username": {username},
		"password": {password},
	})
}

func (s *Session) Signup(username, password, token string) (sm smolboard.Session, err error) {
	return sm, s.Client.Post("/signup", &sm, url.Values{
		"username": {username},
		"password": {password},
		"token":    {token},
	})
}

func (s *Session) Signout() error {
	return s.Client.Post("/signout", nil, nil)
}

func (s *Session) Me() (u smolboard.UserPart, err error) {
	return u, s.Client.Get("/users/@me", &u, nil)
}

func (s *Session) User(username string) (u smolboard.UserPart, err error) {
	return u, s.Client.Get("/users/"+url.PathEscape(username), &u, nil)
}

func (s *Session) Post(id int64) (p smolboard.PostExtended, err error) {
	return p, s.Client.Get(fmt.Sprintf("/posts/%d", id), &p, nil)
}

// Posts returns the paginated post list. Count is defaulted to 25.
func (s *Session) Posts(count, page int) (p smolboard.SearchResults, err error) {
	if count == 0 {
		count = 25
	}

	return p, s.Client.Get("/posts", &p, url.Values{
		"c": {strconv.Itoa(count)},
		"p": {strconv.Itoa(page)},
	})
}

// PostSearch is similar to Posts but with searching.
func (s *Session) PostSearch(q string, count, page int) (p smolboard.SearchResults, err error) {
	if count == 0 {
		count = 25
	}

	return p, s.Client.Get("/posts", &p, url.Values{
		"q": {q},
		"c": {strconv.Itoa(count)},
		"p": {strconv.Itoa(page)},
	})
}

func (s *Session) PostDirectURL(post smolboard.Post) string {
	return fmt.Sprintf("%s/%s/%s", s.Client.Endpoint(), "images", post.Filename())
}

func (s *Session) PostThumbURL(post smolboard.Post) string {
	return fmt.Sprintf("%s/%s/%s/thumb.jpg", s.Client.Endpoint(), "images", post.Filename())
}

func (s *Session) DeletePost(id int64) error {
	return s.Client.Delete(fmt.Sprintf("/posts/%d", id), nil, nil)
}

func (s *Session) SetPostPermission(id int64, p smolboard.Permission) error {
	return s.Client.Request(
		"PATCH",
		fmt.Sprintf("/posts/%d/permission", id),
		nil,
		url.Values{"p": {p.StringInt()}},
	)
}

func (s *Session) TagPost(id int64, tag string) error {
	if err := smolboard.TagIsValid(tag); err != nil {
		return err
	}

	return s.Client.Post(fmt.Sprintf("/posts/%d/tags", id), nil, url.Values{
		"t": {tag},
	})
}

func (s *Session) UntagPost(id int64, tag string) error {
	if err := smolboard.TagIsValid(tag); err != nil {
		return err
	}

	return s.Client.Delete(fmt.Sprintf("/posts/%d/tags", id), nil, url.Values{
		"t": {tag},
	})
}

func (s *Session) ListTokens() (tl smolboard.TokenList, err error) {
	return tl, s.Client.Get("/tokens", &tl, nil)
}

func (s *Session) CreateToken(uses int) (t smolboard.Token, err error) {
	return t, s.Client.Post("/tokens", &t, url.Values{
		"uses": {strconv.Itoa(uses)},
	})
}

func (s *Session) DeleteToken(token string) error {
	return s.Client.Delete("/tokens/"+token, nil, nil)
}
