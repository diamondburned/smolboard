package session

import (
	"database/sql"

	"github.com/diamondburned/smolboard/smolboard/session/internal/db"
	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
)

// Actor is the state for a session. It allows privileged actions with
// permission checks.
type Actor struct {
	database  *Database
	authtoken string
}

var (
	ErrOverUseLimit = httperr.New(400, "requested use is over limit")
)

func (a *Actor) Session() (*db.Session, error) {
	return a.database.Session(a.authtoken)
}

func (a *Actor) AuthToken() string {
	return a.authtoken
}

func (a *Actor) CreateToken(uses int) (*db.Token, error) {
	// Is the user going over the max use limit? If yes, disallow it.
	if uses > a.database.Config.MaxTokenUses {
		return nil, ErrOverUseLimit
	}

	// Is the user requesting for an unlimited use token?
	if uses == -1 {
		// Check permission.
		if err := a.IsPermission(db.PermissionOwner); err != nil {
			return nil, err
		}
	}

	return a.database.CreateToken(uses)
}

func (a *Actor) Permission() (db.Permission, error) {
	tx, err := a.database.Begin()
	if err != nil {
		return 0, errors.Wrap(err, "Failed to start transaction")
	}
	defer tx.Rollback()

	var username string

	r := tx.QueryRow("SELECT username FROM sessions WHERE authtoken = ?", a.authtoken)
	if err := r.Scan(&username); err != nil {
		return 0, errors.Wrap(err, "Failed to get username")
	}

	// Check if the username matches that of the owner's.
	if username == a.database.Config.Owner {
		return db.PermissionOwner, nil
	}

	r = tx.QueryRow("SELECT permission FROM users WHERE username = ?", username)

	var perm db.Permission

	if err := r.Scan(&perm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, db.ErrUserForbidden
		}

		return 0, errors.Wrap(err, "Failed to scan permission")
	}

	return perm, tx.Commit()
}

func (a *Actor) HasPermission(min db.Permission) error {
	p, err := a.Permission()
	if err != nil {
		return err
	}

	// If the user's permission is larger than the required permission, then
	// return no error. This does NOT count if the user has the same permission.
	if p > min {
		return nil
	}

	// Else, return forbidden.
	return db.ErrUserForbidden
}

func (a *Actor) IsPermission(perm db.Permission) error {
	p, err := a.Permission()
	if err != nil {
		return err
	}

	if p == perm {
		return nil
	}

	return db.ErrUserForbidden
}

func (a *Actor) Sessions() ([]db.Session, error) {
	s, err := a.Session()
	if err != nil {
		return nil, err
	}

	return a.database.Sessions(s.Username)
}

// DeleteSessionID deletes the person's own session ID.
func (a *Actor) DeleteSessionID(id int64) error {
	// Start a new transaction to prevent data race.
	tx, err := a.database.Begin()
	if err != nil {
		return errors.Wrap(err, "Failed to start a transaction")
	}
	defer tx.Rollback()

	// As we accept any token, we'd want to validate that it is that user's
	// token.
	var username string
	err = tx.
		QueryRow("SELECT username FROM sessions WHERE authtoken = ?", a.authtoken).
		Scan(&username)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.ErrSessionNotFound
		}

		return errors.Wrap(err, "Failed to get token")
	}

	// Ensure that we are deleting only this user's token.
	_, err = tx.Exec(
		"DELETE FROM sessions WHERE id = ? AND username = ?",
		id, username,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.ErrSessionNotFound
		}

		return errors.Wrap(err, "Failed to delete token")
	}

	return tx.Commit()
}

// Signout removes the session from the database.
func (a *Actor) Signout() error {
	return a.database.Signout(a.authtoken)
}

func (a *Actor) PromoteUser(username string, p db.Permission) error {
	if err := a.HasPermission(p); err != nil {
		return err
	}

	return a.database.SetUserPermission(username, p)
}

func (a *Actor) CreateOwner(password string) (*db.User, error) {
	return a.database.NewUser(a.database.Config.Owner, password, db.PermissionOwner)
}

func (a *Actor) DeleteUser(username string) error {
	// Allow boolean which determines if the user is allowed to delete.
	var allow bool

	// If the username matches, then the user can delete.
	if s, err := a.Session(); err == nil && s.Username == username {
		allow = true
	}

	// If the user is an admin and has a higher permission than the target user,
	// then they can be deleted.

	// Get the other user.
	u, err := a.database.User(username)
	if err != nil {
		return err
	}

	// Compare permissions; see if the current user has higher permissions.
	if err := a.HasPermission(u.Permission); err == nil {
		allow = true
	}

	if !allow {
		return db.ErrUserForbidden
	}

	// Delete allowed; proceed.
	return a.database.DeleteUser(username)
}
