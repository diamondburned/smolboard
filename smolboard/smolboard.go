package smolboard

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/diamondburned/smolboard/server/httperr"
	"github.com/mattn/go-shellwords"
)

// NameIsLegal returns whether or not name contains illegal characters. It
// returns nil if the name does not contain any.
//
// Legal runes include: digits, letters, an underscore and a dash.
func NameIsLegal(name string) error {
	if name == "" {
		return ErrIllegalName
	}

	// testDigitLetter tests if a rune is not a digit or letter. It returns true
	// if that is the case.
	illi := strings.LastIndexFunc(name, func(r rune) bool {
		return !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_')
	})
	if illi > -1 {
		return ErrIllegalName
	}

	return nil
}

// ErrResponse is the structure of the response when the request returns an
// error.
type ErrResponse struct {
	Error string `json:"error"`
}

type Permission int8

var ErrInvalidPermission = errors.New("invalid permission")

const (
	// PermissionGuest is the zero-value of permission, which indicates a guest.
	PermissionGuest Permission = iota
	// PermissionUser is a normal user's base permission. It allows uploading.
	PermissionUser
	// PermissionTrusted has access to posts marked as trusted-only. Trusted
	// users can mark a post as trusted.
	PermissionTrusted
	// PermissionAdministrator can create limited use tokens as well as banning
	// and promoting people up to Trusted. This permission inherits all
	// permissions above.
	PermissionAdministrator
	// PermissionOwner indicates the owner of the image board. The owner can
	// create unlimited tokens and inherits all permissions above. They are also
	// the only person that can promote a person to Administrator.
	PermissionOwner // if username == Owner
)

func (p Permission) String() string {
	switch p {
	case PermissionGuest:
		return "Guest"
	case PermissionUser:
		return "User"
	case PermissionTrusted:
		return "Trusted"
	case PermissionAdministrator:
		return "Administrator"
	case PermissionOwner:
		return "Owner"
	default:
		return "???"
	}
}

type PostAttribute struct {
	Width    int    `json:"w,omitempty"`
	Height   int    `json:"h,omitempty"`
	Blurhash string `json:"blurhash,omitempty"`
}

func (a *PostAttribute) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	if b, ok := v.([]byte); ok {
		return json.Unmarshal(b, a)
	}

	return fmt.Errorf("Failed to scan %#v: unexpected type", v)
}

func (a PostAttribute) Value() (driver.Value, error) {
	return json.Marshal(a)
}

type Post struct {
	ID          int64         `json:"id"          db:"id"`
	Poster      *string       `json:"poster"      db:"poster"`
	ContentType string        `json:"contenttype" db:"contenttype"`
	Permission  Permission    `json:"permission"  db:"permission"`
	Attributes  PostAttribute `json:"attributes"  db:"attributes"`
}

var (
	ErrMissingExt          = httperr.New(400, "file does not have extension")
	ErrPostNotFound        = httperr.New(404, "post not found")
	ErrPageCountLimit      = httperr.New(400, "count is over 100 limit")
	ErrUnsupportedFileType = httperr.New(415, "unsupported file type")
)

func (p *Post) SetPoster(poster string) {
	cpy := poster
	p.Poster = &cpy
}

func (p *Post) GetPoster() string {
	if p.Poster == nil {
		return ""
	}
	return *p.Poster
}

// Filename returns the filename that the file should be written to.
func (p Post) Filename() string {
	var sid = strconv.FormatInt(p.ID, 10)

	var parts = strings.Split(p.ContentType, "/")
	if len(parts) < 2 {
		return sid
	}

	return fmt.Sprintf("%s.%s", sid, parts[1])
}

// PostWithTags is the type for a post with queried tags.
type PostWithTags struct {
	Post
	// Tags is manually queried externally.
	Tags []PostTag `json:"tags"`
}

type PostTag struct {
	PostID  int64  `db:"postid"  json:"post_id,omitempty"`
	TagName string `db:"tagname" json:"tag_name,omitempty"`
	Count   int    `db:"-"       json:"count,omitempty"`
}

const MaxTagLen = 256

var (
	ErrEmptyTag        = httperr.New(400, "empty tag not allowed")
	ErrIllegalTag      = httperr.New(400, "tag contains illegal character")
	ErrTagTooLong      = httperr.New(400, "tag is too long")
	ErrTagAlreadyAdded = httperr.New(400, "tag is already added")
)

// TagIsValid returns nil if the tag is valid else an error. A tag is invalid if
// it's empty, it's longer than 256 bytes, it's prefixed with an at sign "@" or
// it contains anything not a graphical character defined by the Unicode
// standards.
func TagIsValid(tagName string) error {
	if tagName == "" {
		return ErrEmptyTag
	}
	if len(tagName) > 256 {
		return ErrTagTooLong
	}

	if strings.HasPrefix(tagName, "@") {
		return ErrIllegalTag
	}

	illi := strings.LastIndexFunc(tagName, func(r rune) bool {
		return !(unicode.IsGraphic(r))
	})
	if illi > -1 {
		return ErrIllegalTag
	}
	return nil
}

// Query represents the parsed query string. A zero-value PostQuery searches
// for nothing and thus will list all posts.
type Query struct {
	Poster string
	Tags   []string
}

var (
	ErrQueryAlreadyHasUser = httperr.New(400, "search query already has a user filter")
)

// AllPosts searches for all posts; it is a zero value instance of PostQuery.
var AllPosts = Query{}

// SearchResults is the results returned from the queried posts.
type SearchResults struct {
	// Total is the total number of
	Total int    `json:"total"`
	Posts []Post `json:"posts"`
}

// NoResults contains no search results; it is a zero value instance of
// PostQueryResults.
var NoResults = SearchResults{}

// ParsePostQuery parses a search string to query the post gallery. The syntax
// is space-delimited optionally quoted tags with an optional prefix in front to
// indicate a post author. A post author search may only appear once. Below is
// an example:
//
//     tag1 "tag with space" 'more spaces' @diamondburned
//
func ParsePostQuery(q string) (Query, error) {
	// Fast path.
	if q == "" {
		return AllPosts, nil
	}

	words, err := shellwords.Parse(q)
	if err != nil {
		return AllPosts, httperr.Wrap(err, 400, "Invalid query")
	}

	var tags = words[:0]
	var targetUser = ""

	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			// Disallow query with multiple users and error out.
			if targetUser != "" {
				return AllPosts, ErrQueryAlreadyHasUser
			}

			// Only allow non-empty mentions; ignore all random ats.
			if user := strings.TrimPrefix(word, "@"); user != "" {
				targetUser = user
			}
		} else {
			// Make sure the tag is legal before re-adding.
			if err := TagIsValid(word); err != nil {
				return AllPosts, err
			}
			tags = append(tags, word)
		}
	}

	return Query{
		Poster: targetUser,
		Tags:   tags,
	}, nil
}

var wordEscaper = strings.NewReplacer(`'`, `\'`)

// String encodes the parsed PostQuery to a regular string query.
func (q Query) String() string {
	var b strings.Builder

	if q.Poster != "" {
		b.WriteByte('@')
		b.WriteString(q.Poster)
		b.WriteByte(' ')
	}

	for i, tag := range q.Tags {
		if i != 0 {
			b.WriteByte(' ')
		}

		if strings.Contains(tag, " ") {
			b.WriteByte('\'')
			wordEscaper.WriteString(&b, tag)
			b.WriteByte('\'')
		} else {
			b.WriteString(tag)
		}
	}

	return b.String()
}

type Session struct {
	ID       int64  `json:"-" db:"id"`
	Username string `json:"-" db:"username"`
	// AuthToken is the token stored in the cookies.
	AuthToken string `json:"authtoken" db:"authtoken"`
	// Deadline is gradually updated with each Session call, which is per
	// request.
	Deadline int64 `json:"deadline" db:"deadline"`
	// UserAgent is obtained once on login.
	UserAgent string `json:"-" db:"useragent"`
}

var (
	ErrSessionNotFound = httperr.New(401, "session not found")
	ErrSessionExpired  = httperr.New(410, "session expired")
)

// IsZero returns true if the session is a guest one.
func (s Session) IsZero() bool {
	return s.ID == 0
}

type Token struct {
	Token     string `json:"token"     db:"token"`
	Creator   string `json:"creator"   db:"creator"`
	Remaining int    `json:"remaining" db:"remaining"`
}

var (
	ErrUnknownToken = httperr.New(401, "unknown token")
	ErrOverUseLimit = httperr.New(400, "requested use is over limit")
)

// HashCost controls the bcrypt hash cost.
const HashCost = 12

// MinimumPassLength defines the minimum length of a password.
const MinimumPassLength = 8

// UserPart contains non-sensitive parts about the user.
type UserPart struct {
	Username   string     `db:"username"`
	Permission Permission `db:"permission"`
}

type User struct {
	UserPart
	Passhash []byte `db:"passhash"`
}

var (
	ErrOwnerAccountStays  = httperr.New(400, "owner account stays")
	ErrActionNotPermitted = httperr.New(403, "action not permitted")
	ErrUserNotFound       = httperr.New(404, "user not found")
	ErrInvalidPassword    = httperr.New(401, "invalid password")
	ErrPasswordTooShort   = httperr.New(400, "password too short")
	ErrUsernameTaken      = httperr.New(409, "username taken")
	ErrIllegalName        = httperr.New(403, "username contains illegal characters")
)
