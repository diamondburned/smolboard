package db

import (
	"database/sql"
	"strings"
	"unicode"

	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

type Permission uint8

var ErrInvalidPermission = errors.New("invalid permission")

const (
	// PermissionNormal is a normal user's base permission. It allows uploading.
	PermissionNormal Permission = iota
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

// HashCost controls the bcrypt hash cost.
const HashCost = 12

// MinimumPassLength defines the minimum length of a password.
const MinimumPassLength = 8

type User struct {
	Username   string     `db:"username"`
	Passhash   []byte     `db:"passhash"`
	Permission Permission `db:"permission"`
}

var (
	ErrUserForbidden    = httperr.New(403, "action not permitted")
	ErrUserNotFound     = httperr.New(404, "user not found")
	ErrInvalidPassword  = httperr.New(401, "invalid password")
	ErrPasswordTooShort = httperr.New(400, "password too short")
	ErrIllegalName      = httperr.New(403, "username contains illegal characters")
)

func (u User) VerifyPassword(password string) error {
	err := bcrypt.CompareHashAndPassword(u.Passhash, []byte(password))

	// If the error is a mismatch, then we return an invalid password.
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidPassword
	}

	// Wrap will return nil if the error is nil.
	return errors.Wrap(err, "Failed to compare password")
}

func (d *Database) NewUser(username, password string, perm Permission) (*User, error) {
	if !nameAllowed(username) {
		return nil, ErrIllegalName
	}

	if len(password) < MinimumPassLength {
		return nil, ErrPasswordTooShort
	}

	p, err := bcrypt.GenerateFromPassword([]byte(password), HashCost)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate password")
	}

	user := User{
		Username:   username,
		Passhash:   p,
		Permission: perm,
	}

	_, err = d.Exec(
		"INSERT INTO users VALUES (?, ?, ?)",
		user.Username, user.Passhash, user.Permission,
	)

	// Do the magical Go way of asserting errors.
	if err != nil {
		sqlerr := sqlite3.Error{}

		// Unique constraint means we're attempting to make a username that's
		// colliding. We could return an error close to that.
		if errors.As(err, &sqlerr) && sqlerr.Code == sqlite3.ErrConstraint {
			return nil, ErrIllegalName
		}

		return nil, errors.Wrap(err, "Failed to insert user")
	}

	return &user, nil
}

func (d *Database) User(username string) (*User, error) {
	if !nameAllowed(username) {
		return nil, ErrUserNotFound
	}

	u := User{Username: username}
	r := d.QueryRowx("SELECT passhash, permission FROM users WHERE username = ?", username)

	err := r.Scan(&u.Passhash, &u.Permission)
	if err == nil {
		return &u, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	return nil, errors.Wrap(err, "Failed to scan row to user")
}

// TODO: change password

func (d *Database) DeleteUser(username string) error {
	if !nameAllowed(username) {
		return ErrIllegalName
	}

	_, err := d.Exec("DELETE FROM users WHERE username = ?", username)
	return err
}

func (d *Database) SetUserPermission(username string, p Permission) error {
	if p < 0 || p > PermissionAdministrator {
		return ErrInvalidPermission
	}

	_, err := d.Exec("UPDATE users SET permission = ? WHERE username = ?", p, username)
	if err != nil {
		return errors.Wrap(err, "Failed to save changes")
	}

	return nil
}

func nameAllowed(name string) bool {
	if name == "" {
		return false
	}

	return strings.LastIndexFunc(name, testDigitLetter) == -1
}

// testDigitLetter tests if a rune is not a digit or letter. It returns true if
// that is the case.
func testDigitLetter(r rune) bool {
	return !(unicode.IsDigit(r) || unicode.IsLetter(r))
}
