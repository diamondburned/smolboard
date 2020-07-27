package db

import (
	"database/sql"
	"strings"
	"unicode"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

func VerifyPassword(hash []byte, password string) error {
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))

	// If the error is a mismatch, then we return an invalid password.
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return smolboard.ErrInvalidPassword
	}

	// Wrap will return nil if the error is nil.
	return errors.Wrap(err, "Failed to compare password")
}

// createUser creates a new user in the database with the given username. The
// username that's inserted into the database is guaranteed to be the same as
// the argument.
func (d *Transaction) createUser(username, password string, perm smolboard.Permission) error {
	if !nameAllowed(username) {
		return smolboard.ErrIllegalName
	}

	if len(password) < smolboard.MinimumPassLength {
		return smolboard.ErrPasswordTooShort
	}

	p, err := bcrypt.GenerateFromPassword([]byte(password), smolboard.HashCost)
	if err != nil {
		return errors.Wrap(err, "Failed to generate password")
	}

	_, err = d.Exec("INSERT INTO users VALUES (?, ?, ?)", username, p, perm)
	if err != nil {
		// Unique constraint means we're attempting to make a username that's
		// colliding. We could return an error close to that.
		if errIsConstraint(err) {
			return smolboard.ErrUsernameTaken
		}

		return errors.Wrap(err, "Failed to insert user")
	}

	return nil
}

// User returns the user WITHOUT the passhash.
func (d *Transaction) User(username string) (*smolboard.UserPart, error) {
	if !nameAllowed(username) {
		return nil, smolboard.ErrUserNotFound
	}

	u := smolboard.UserPart{Username: username}
	r := d.QueryRowx("SELECT permission FROM users WHERE username = ?", username)

	err := r.Scan(&u.Permission)
	if err == nil {
		return &u, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, smolboard.ErrUserNotFound
	}

	return nil, errors.Wrap(err, "Failed to scan row to user")
}

// PromoteUser promotes or demotes someone else.
func (d *Transaction) PromoteUser(username string, p smolboard.Permission) error {
	// No inclusivity, as the current user can only promote to the permission
	// below them. This way, only the owner can promote users to admins, and
	// only admins can promote users to trusted. Trusted users can't promote.
	if err := d.HasPermission(p, false); err != nil {
		return err
	}

	// The larger-than check is redundant, as nobody should ever have a higher
	// permission than Owner, and Owner cannot make anyone else Owner, as
	// HasPermission checks for less-than.
	if p < 0 || p > smolboard.PermissionAdministrator {
		return smolboard.ErrInvalidPermission
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
	// Behavior:
	//    Current Admin, target Trusted -> delete
	//    Current Admin, target Admin   -> no
	//    Current Admin, target is self and Admin -> delete
	//    Current Trusted, target user -> no
	//    Current Trusted, target Trusted -> no
	//    Current Trusted, target is self and Trusted -> delete
	if err := d.IsUserOrHasPermOver(smolboard.PermissionAdministrator, username); err != nil {
		return err
	}

	// Prevent deletion of the owner account. We do this check last to
	// prioritize permission errors.
	if d.config.Owner == username {
		return smolboard.ErrOwnerAccountStays
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
