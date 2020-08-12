package client

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/diamondburned/smolboard/smolboard"
)

// Session is the current smolboard HTTP session.
type Session struct {
	Client *Client
}

// NewSession creates a new session with the given endpoint.
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

// AllowedTypes returns a list of allowed MIME types.
func (s *Session) AllowedTypes() (ts []string, err error) {
	return ts, s.Client.Get("/filetypes", &ts, nil)
}

// Endpoint returns the current endpoint with the appended path. The path is
// optional, but it must start with a slash ("/") otherwise.
func (s *Session) Endpoint(path string) string {
	return s.Client.Endpoint() + path
}

// Signin signs a new user in.
func (s *Session) Signin(username, password string) (sm smolboard.Session, err error) {
	return sm, s.Client.Post("/signin", &sm, url.Values{
		"username": {username},
		"password": {password},
	})
}

// Signup registers a new user.
func (s *Session) Signup(username, password, token string) (sm smolboard.Session, err error) {
	return sm, s.Client.Post("/signup", &sm, url.Values{
		"username": {username},
		"password": {password},
		"token":    {token},
	})
}

// Signout signs the current user out, which will invalidate their current
// token.
func (s *Session) Signout() error {
	return s.Client.Post("/signout", nil, nil)
}

// Me returns the current user.
func (s *Session) Me() (u smolboard.UserPart, err error) {
	return u, s.Client.Get("/users/@me", &u, nil)
}

// UserEditParams is the parameters for EditMe. All fields are optional.
type UserEditParams struct {
	Password string
}

// EditMe edits the current user. All parameters in the given UserEditParams are
// optional with their respective zero values.
func (s *Session) EditMe(pp UserEditParams) (u smolboard.UserPart, err error) {
	return u, s.Client.Request("PATCH", "/users/@me", &u, url.Values{
		"password": {pp.Password},
	})
}

// DeleteMe deletes the currenet user.
func (s *Session) DeleteMe() error {
	return s.Client.Delete("/users/@me", nil, nil)
}

// User gets a user with the given username.
func (s *Session) User(username string) (u smolboard.UserPart, err error) {
	return u, s.Client.Get("/users/"+url.PathEscape(username), &u, nil)
}

// Users gets a paginated list of users. The default value for count is 50. This
// endpoint is only allowed for the owner and admins.
func (s *Session) Users(count, page int) (u smolboard.UserList, err error) {
	return s.SearchUsers("", count, page)
}

// SearchUsers searches for a user. The default values from Users applies here
// as well. If q is empty, then all users are listed.
func (s *Session) SearchUsers(q string, count, page int) (u smolboard.UserList, err error) {
	if count == 0 {
		count = 50
	}

	return u, s.Client.Get("/users", &u, url.Values{
		"q": {q},
		"c": {strconv.Itoa(count)},
		"p": {strconv.Itoa(page)},
	})
}

// DeleteUser deletes a user.
func (s *Session) DeleteUser(username string) error {
	return s.Client.Delete("/users/"+url.PathEscape(username), nil, nil)
}

// Post returns a post with the given ID. It returns extra information such as
// tags.
func (s *Session) Post(id int64) (p smolboard.PostExtended, err error) {
	return p, s.Client.Get(fmt.Sprintf("/posts/%d", id), &p, nil)
}

// Posts returns the paginated post list. Count is defaulted to 25.
func (s *Session) Posts(count, page int) (p smolboard.SearchResults, err error) {
	return s.PostSearch("", count, page)
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

// PostDirectURL returns the direct URL to the post's content.
func (s *Session) PostDirectURL(post smolboard.Post) string {
	return fmt.Sprintf("%s/%s/%s", s.Client.Endpoint(), "images", post.Filename())
}

// PostThumbURL returns the JPEG URL to the thumbnail of the given post.
func (s *Session) PostThumbURL(post smolboard.Post) string {
	return fmt.Sprintf("%s/%s/%s/thumb.jpg", s.Client.Endpoint(), "images", post.Filename())
}

// DeletePost deletes the given post.
func (s *Session) DeletePost(id int64) error {
	return s.Client.Delete(fmt.Sprintf("/posts/%d", id), nil, nil)
}

// SetPostPermission sets the given post's permission.
func (s *Session) SetPostPermission(postID int64, p smolboard.Permission) error {
	return s.Client.Request(
		"PATCH",
		fmt.Sprintf("/posts/%d/permission", postID),
		nil,
		url.Values{"p": {p.StringInt()}},
	)
}

// TagPost adds a tag to a post.
func (s *Session) TagPost(postID int64, tag string) error {
	if err := smolboard.TagIsValid(tag); err != nil {
		return err
	}

	return s.Client.Post(fmt.Sprintf("/posts/%d/tags", postID), nil, url.Values{
		"t": {tag},
	})
}

// UntagPost removes a tag from a post.
func (s *Session) UntagPost(postID int64, tag string) error {
	if err := smolboard.TagIsValid(tag); err != nil {
		return err
	}

	return s.Client.Delete(fmt.Sprintf("/posts/%d/tags", postID), nil, url.Values{
		"t": {tag},
	})
}

// Tokens returns a list of tokens along with extra bits returned from the
// server to assist in getting information without extra queries.
func (s *Session) Tokens() (tl smolboard.TokenList, err error) {
	return tl, s.Client.Get("/tokens", &tl, nil)
}

// CreateToken creates a token with the given uses count. This function returns
// ErrZeroNotAllowed if uses is 0.
func (s *Session) CreateToken(uses int) (t smolboard.Token, err error) {
	if uses == 0 {
		return t, smolboard.ErrZeroNotAllowed
	}

	return t, s.Client.Post("/tokens", &t, url.Values{
		"uses": {strconv.Itoa(uses)},
	})
}

// DeleteToken deletes the token with the given token string.
func (s *Session) DeleteToken(token string) error {
	return s.Client.Delete("/tokens/"+token, nil, nil)
}

// GetSessions returns a list of sessions.
func (s *Session) GetSessions() (ses []smolboard.Session, err error) {
	return ses, s.Client.Get("/users/@me/sessions", &ses, nil)
}

// Session returns the session with the given ID.
func (s *Session) Session(id int64) (ses smolboard.Session, err error) {
	return ses, s.Client.Get(fmt.Sprintf("/users/@me/sessions/%d", id), &ses, nil)
}

// CurrentSession returns the current session.
func (s *Session) CurrentSession() (ses smolboard.Session, err error) {
	return ses, s.Client.Get("/users/@me/sessions/@this", &ses, nil)
}

// DeleteSession deletes the session with the given ID.
func (s *Session) DeleteSession(id int64) error {
	return s.Client.Delete(fmt.Sprintf("/users/@me/sessions/%d", id), nil, nil)
}

// DeleteCurrentSession deletes the current session.
func (s *Session) DeleteCurrentSession() error {
	return s.Client.Delete("/users/@me/sessions/@this", nil, nil)
}

// DeleteAllSessions deletes all sessions except for the current one.
func (s *Session) DeleteAllSessions() error {
	return s.Client.Delete("/users/@me/sessions", nil, nil)
}
