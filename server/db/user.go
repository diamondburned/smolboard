package db

import (
	"database/sql"
	"time"

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
	if err := smolboard.NameIsLegal(username); err != nil {
		return err
	}

	if len(password) < smolboard.MinimumPassLength {
		return smolboard.ErrPasswordTooShort
	}

	p, err := bcrypt.GenerateFromPassword([]byte(password), smolboard.HashCost)
	if err != nil {
		return errors.Wrap(err, "Failed to generate password")
	}

	_, err = d.Exec(
		"INSERT INTO users VALUES (?, ?, ?, ?)",
		username, time.Now().UnixNano(), p, perm,
	)

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

func (d *Transaction) Users(count, page uint) (smolboard.UserList, error) {
	return d.SearchUsers("", count, page)
}

func (d *Transaction) SearchUsers(qs string, count, page uint) (smolboard.UserList, error) {
	if count > 100 {
		return smolboard.NoUsers, smolboard.ErrPageCountLimit
	}

	p, err := d.Permission()
	if err != nil {
		return smolboard.NoUsers, err
	}

	// Only admins and higherups can list users.
	if err := p.HasPermission(smolboard.PermissionAdministrator, true); err != nil {
		return smolboard.NoUsers, err
	}

	var list = smolboard.UserList{
		Users: make([]smolboard.UserPart, 0, count),
	}

	// Hack to list all users if the current user is an owner.
	if p == smolboard.PermissionOwner {
		p++
	}

	r := d.QueryRow(`
		SELECT COUNT(1) FROM users WHERE permission < ? AND username LIKE ? || '%'`,
		p, qs,
	)

	if err := r.Scan(&list.Total); err != nil {
		return smolboard.NoUsers, errors.Wrap(err, "Failed to scan total")
	}

	// Only show users whose permissions are lower than the current user.
	q, err := d.Queryx(`
		SELECT * FROM users WHERE permission < ? AND username LIKE ? || '%'
			ORDER BY jointime ASC, permission DESC LIMIT ?, ?`,
		p, qs, count*page, count,
	)

	if err != nil {
		return smolboard.NoUsers, errors.Wrap(err, "Failed to get users")
	}

	defer q.Close()

	for q.Next() {
		var u smolboard.User

		if err := q.StructScan(&u); err != nil {
			return smolboard.NoUsers, errors.Wrap(err, "Failed to scan user")
		}

		list.Users = append(list.Users, u.UserPart)
	}

	return list, nil
}

// User returns the user WITHOUT the passhash.
func (d *Transaction) User(username string) (*smolboard.UserPart, error) {
	if err := smolboard.NameIsLegal(username); err != nil {
		// Illegal name is a non-existent name.
		return nil, smolboard.ErrUserNotFound
	}

	u := smolboard.UserPart{Username: username}
	r := d.QueryRowx("SELECT jointime, permission FROM users WHERE username = ?", username)

	if err := r.Scan(&u.JoinTime, &u.Permission); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, smolboard.ErrUserNotFound
		}

		return nil, errors.Wrap(err, "Failed to scan row to user")
	}

	// Override the permission without actually overriding the database if
	// needed.
	if d.config.Owner == username {
		u.Permission = smolboard.PermissionOwner
	}

	return &u, nil
}

func (d *Transaction) Me() (*smolboard.UserPart, error) {
	return d.User(d.Session.Username)
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

func (d *Transaction) ChangePassword(password string) error {
	if len(password) < smolboard.MinimumPassLength {
		return smolboard.ErrPasswordTooShort
	}

	p, err := bcrypt.GenerateFromPassword([]byte(password), smolboard.HashCost)
	if err != nil {
		return errors.Wrap(err, "Failed to generate password")
	}

	_, err = d.Exec("UPDATE users SET passhash = ? WHERE username = ?", p, d.Session.Username)
	if err != nil {
		return errors.Wrap(err, "Failed to change password")
	}

	if err := d.DeleteAllSessions(); err != nil {
		return errors.Wrap(err, "Failed to invalidate other sessions")
	}

	return nil
}

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
