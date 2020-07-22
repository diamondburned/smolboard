package db

import (
	"database/sql"
	"strings"
	"unicode"

	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
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

func VerifyPassword(hash []byte, password string) error {
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))

	// If the error is a mismatch, then we return an invalid password.
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidPassword
	}

	// Wrap will return nil if the error is nil.
	return errors.Wrap(err, "Failed to compare password")
}

func NewUser(username, password string, perm Permission) (*User, error) {
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

	return &User{
		UserPart: UserPart{username, perm},
		Passhash: p,
	}, nil
}

func (u *User) insert(tx *sql.Tx) error {
	_, err := tx.Exec(
		"INSERT INTO users VALUES (?, ?, ?)",
		u.Username, u.Passhash, u.Permission,
	)

	// Do the magical Go way of asserting errors.
	if err != nil {
		// Unique constraint means we're attempting to make a username that's
		// colliding. We could return an error close to that.
		if errIsConstraint(err) {
			return ErrUsernameTaken
		}

		return errors.Wrap(err, "Failed to insert user")
	}

	return nil
}

// User returns the user WITHOUT the passhash.
func (d *Transaction) User(username string) (*UserPart, error) {
	if !nameAllowed(username) {
		return nil, ErrUserNotFound
	}

	u := UserPart{Username: username}
	r := d.QueryRowx("SELECT permission FROM users WHERE username = ?", username)

	err := r.Scan(&u.Permission)
	if err == nil {
		return &u, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	return nil, errors.Wrap(err, "Failed to scan row to user")
}

// PromoteUser promotes or demotes someone else.
func (d *Transaction) PromoteUser(username string, p Permission) error {
	// No inclusivity, as the current user can only promote to the permission
	// below them. This way, only the owner can promote users to admins, and
	// only admins can promote users to trusted. Trusted users can't promote.
	if err := d.HasPermission(p, false); err != nil {
		return err
	}

	// The larger-than check is redundant, as nobody should ever have a higher
	// permission than Owner, and Owner cannot make anyone else Owner, as
	// HasPermission checks for less-than.
	if p < 0 || p > PermissionAdministrator {
		return ErrInvalidPermission
	}

	_, err := d.Exec("UPDATE users SET permission = ? WHERE username = ?", p, username)
	if err != nil {
		return errors.Wrap(err, "Failed to save changes")
	}

	return nil
}

// TODO: change password

// TODO add tests to confirm Trusted cannot delete Normal
// TODO add tests to confirm Admin cannot delete Owner

func (d *Transaction) DeleteUser(username string) error {
	// Make sure the user performing this action is either the user being
	// deleted or an administrator.
	if err := d.HasPermOrIsUser(PermissionAdministrator, username, true); err != nil {
		return err
	}

	// Prevent deletion of the owner account. We do this check last to
	// prioritize permission errors.
	if d.config.Owner == username {
		return ErrOwnerAccountStays
	}

	_, err := d.Exec("DELETE FROM users WHERE username = ?", username)
	return err
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
