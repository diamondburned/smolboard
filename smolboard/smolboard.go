package smolboard

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/snowflake"
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

	// reserved
	permissionLen
)

var permissions = func() []Permission {
	var permissions = make([]Permission, permissionLen)
	for i := Permission(0); i < permissionLen; i++ {
		permissions[i] = i
	}
	return permissions
}()

// AllPermissions returns all possible permissions in a slice. It starts with
// the lowest (guest) and ends with the highest (owner).
func AllPermissions() []Permission {
	return permissions
}

// Int returns the permission as a stringed integer enum. This should only be
// used for form values and the like.
func (p Permission) StringInt() string {
	return strconv.Itoa(int(p))
}

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

func (p Permission) HasPermission(min Permission, inclusive bool) error {
	// Is this a valid permission?
	if min < PermissionGuest || min > PermissionOwner {
		return ErrInvalidPermission
	}

	if p > min || (inclusive && p == min) {
		return nil
	}

	// Else, return forbidden.
	return ErrActionNotPermitted
}

func (p Permission) IsUserOrHasPermOver(min, target Permission, self, targetUser string) error {
	if self == targetUser {
		return nil
	}
	return p.HasPermOverUser(min, target, self, targetUser)
}

// HasPermOverUser returns nil if the current user (self) has permission over
// the target user with the given lowest permission required. If target is -1,
// then PermissionAdministrator is assumed. This is done for deleted accounts.
func (p Permission) HasPermOverUser(min, target Permission, self, targetUser string) error {
	// Is this a valid permission?
	if min < PermissionGuest || min > PermissionOwner {
		return ErrInvalidPermission
	}

	// If the target permission is the same or larger than the current user's
	// permission and the user is different, then reject.
	if p < min {
		return ErrActionNotPermitted
	}

	// If the target user is the current user and the target permission is the
	// same or lower than the target, then allow.
	if self == targetUser && p >= min {
		return nil
	}

	// Set a default target permission if it's not valid.
	if target < 0 {
		target = PermissionAdministrator
	}

	// At this point, p >= min. This means the user does indeed have more than
	// the required requirements. We now need to check that the target
	// permission has a lower permission.

	// If the target user has the same or higher permission, then deny.
	if target >= p {
		return ErrActionNotPermitted
	}

	// At this point:
	// 1. The current user has more or same permission than what's required.
	// 2. The target has a lower permission than the current user.
	// 3. The target user is not the current user.
	return nil
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
	ID          int64         `json:"id"           db:"id"`
	Size        int64         `json:"size"         db:"size"`
	Poster      *string       `json:"poster"       db:"poster"`
	ContentType string        `json:"content_type" db:"contenttype"`
	Permission  Permission    `json:"permission"   db:"permission"`
	Attributes  PostAttribute `json:"attributes"   db:"attributes"`
}

var (
	ErrMissingExt     = httperr.New(400, "file does not have extension")
	ErrPostNotFound   = httperr.New(404, "post not found")
	ErrPageCountLimit = httperr.New(400, "count is over 100 limit")
)

// SetPoster sets the post's poster.
func (p *Post) SetPoster(poster string) {
	cpy := poster
	p.Poster = &cpy
}

// GetPoster returns an empty string if the poster is nil, or the poster's name
// already dereferenced if not.
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

// CreatedTime returns the time the post was created. It's milliseconds
// accurate.
func (p Post) CreatedTime() time.Time {
	const ms = int64(time.Millisecond)
	return time.Unix(0, snowflake.ID(p.ID).Time()*ms)
}

// PostExtended is the type for a post with queried tags and the poster user.
// This struct is returned from /posts/:id.
type PostExtended struct {
	Post
	// PosterUser is non-nil if the poster is not null.
	PosterUser *UserPart `json:"poster_user"`
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

// QueryTagLimit is the maximum number of tags allowed in a single query.
const QueryTagLimit = 1024

var (
	ErrQueryAlreadyHasUser = httperr.New(400, "search query already has a user filter")
	ErrQueryHasTooMayTags  = httperr.New(400, "search query has too many tags")
)

// AllPosts searches for all posts; it is a zero value instance of PostQuery.
var AllPosts = Query{}

// SearchResults is the results returned from the queried posts.
type SearchResults struct {
	// Posts contains the paginated list of posts.
	Posts []Post `json:"posts"`
	// Total is the total number of posts found.
	Total int `json:"total"`
	// Sizes is the total size of all posts found.
	Sizes int64 `json:"sizes"`
	// User is the user stated in the search query. It is nil if there's no user
	// stated.
	User *UserPart `json:"user,omitempty"`
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

			// Exit if there are too many tag filters.
			if len(tags) > QueryTagLimit {
				return AllPosts, ErrQueryHasTooMayTags
			}
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

type TokenList struct {
	Tokens  []Token
	MaxUses int
}

type Token struct {
	Token     string `json:"token"     db:"token"`
	Creator   string `json:"creator"   db:"creator"`
	Remaining int    `json:"remaining" db:"remaining"`
}

var (
	ErrUnknownToken   = httperr.New(401, "unknown token")
	ErrOverUseLimit   = httperr.New(400, "requested use is over limit")
	ErrZeroNotAllowed = httperr.New(400, "zero use not allowed")
)

// HashCost controls the bcrypt hash cost.
const HashCost = 12

// MinimumPassLength defines the minimum length of a password.
const MinimumPassLength = 8

// UserPart contains non-sensitive parts about the user.
type UserPart struct {
	Username   string     `db:"username"   json:"username"`
	JoinTime   int64      `db:"jointime"   json:"join_time"`
	Permission Permission `db:"permission" json:"permission"`
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

// Joined returns the time the user joined.
func (u UserPart) Joined() time.Time {
	return time.Unix(0, u.JoinTime)
}

// CanChangePost returns nil if the user can change the given post. This is kept
// in sync with the backend functions.
func (u UserPart) CanChangePost(p Post) error {
	return u.Permission.IsUserOrHasPermOver(
		PermissionAdministrator, p.Permission, u.Username, p.GetPoster(),
	)
}

// CanSetPostPermission returns nil if the user can change the post's permission
// to target. This is kept in sync with the backend functions.
func (u UserPart) CanSetPostPermission(p PostExtended, target Permission) error {
	var posterPerm = Permission(-1)
	if p.PosterUser != nil {
		posterPerm = p.PosterUser.Permission
	}

	return u.Permission.HasPermOverUser(target, posterPerm, u.Username, p.GetPoster())
}
