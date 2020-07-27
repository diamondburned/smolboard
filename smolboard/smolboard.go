package smolboard

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"strconv"

	"github.com/diamondburned/smolboard/server/httperr"
)

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

	t, err := mime.ExtensionsByType(p.ContentType)
	if err != nil || len(t) == 0 {
		return sid
	}

	return sid + t[0]
}

// PostWithTags is the type for a post with queried tags.
type PostWithTags struct {
	Post
	// Tags is manually queried externally.
	Tags []PostTag `json:"tags"`
}

type PostTag struct {
	PostID  int64  `json:"post_id"  db:"postid"`
	TagName string `json:"tag_name" db:"tagname"`
	Count   int    `json:"count"    db:"-"`
}

const MaxTagLen = 256

var (
	ErrEmptyTag        = httperr.New(400, "empty tag not allowed")
	ErrTagTooLong      = httperr.New(400, "tag is too long")
	ErrTagAlreadyAdded = httperr.New(400, "tag is already added")
)

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
