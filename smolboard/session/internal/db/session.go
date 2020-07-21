package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"time"

	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
)

type Session struct {
	ID       int64  `db:"id"`
	Username string `db:"username"`
	// AuthToken is the token stored in the cookies.
	AuthToken string `db:"authtoken"`
	// Deadline is gradually updated with each Session call, which is per
	// request.
	Deadline int64 `db:"deadline"`
	// UserAgent is obtained once on login.
	UserAgent string `db:"useragent"`
}

var (
	ErrSessionNotFound = httperr.New(401, "session not found")
	ErrSessionExpired  = httperr.New(410, "session expired")
)

// Signin creates a new session using the given username and password. The
// UserAgent will be used for listing sessions. This function returns an
// authenticate token.
func (d *Database) Signin(username, password, userAgent string) (*Session, error) {
	u, err := d.User(username)
	if err != nil {
		// Return an invalid password for a non-existent user.
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidPassword
		}

		return nil, err
	}

	if err := u.VerifyPassword(password); err != nil {
		return nil, err
	}

	return d.newSession(username, userAgent)
}

// Signup creates a new account using the given username, password and token.
// The token comes from token.go, which is basically an invitation ticket. This
// function retunrs an authenticate token.
func (d *Database) Signup(user, pass, token, userAgent string) (*Session, error) {
	// Verify the token.
	if err := d.UseToken(token); err != nil {
		return nil, err
	}

	_, err := d.NewUser(user, pass, PermissionNormal)
	if err != nil {
		return nil, err
	}

	return d.newSession(user, userAgent)
}

func (d *Database) Signout(token string) error {
	_, err := d.Exec("DELETE FROM sessions WHERE authtoken = ?", token)
	return err
}

// session fetches the session with a renewed deadline.
func (d *Database) Session(token string) (*Session, error) {
	s := Session{AuthToken: token}

	// Start a new transaction to prevent data race.
	tx, err := d.Beginx()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to start a transaction")
	}
	defer tx.Rollback()

	err = tx.
		QueryRowx("SELECT * FROM sessions WHERE authtoken = ?", token).
		StructScan(&s)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}

	var now = time.Now()

	// If the token is expired, then (try to) delete it and return the expired
	// error.
	if now.UnixNano() > s.Deadline {
		return nil, ErrSessionExpired
	}

	// Bump up the expiration time.
	now = now.Add(d.Config.tokenLifespan)
	s.Deadline = now.UnixNano()

	_, err = tx.Exec(
		"UPDATE sessions SET deadline = ? WHERE authtoken = ?",
		s.Deadline, token,
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to renew token")
	}

	return &s, nil
}

func (d *Database) Sessions(username string) ([]Session, error) {
	r, err := d.Queryx("SELECT * FROM sessions WHERE username = ?", username)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query for sessions")
	}

	var sessions []Session

	for r.Next() {
		var s Session

		if err := r.StructScan(&s); err != nil {
			return nil, errors.Wrap(err, "Failed to scan to a session")
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

func (d *Database) CleanupSessions(now int64) error {
	_, err := d.Exec("DELETE FROM sessions WHERE deadline < ?", now)
	return err
}

func (d *Database) newSession(username, userAgent string) (*Session, error) {
	t, err := randToken()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate a token")
	}

	s := Session{
		ID:        int64(d.sf.Generate()),
		Username:  username,
		AuthToken: t,
		Deadline:  time.Now().Add(d.Config.tokenLifespan).UnixNano(),
		UserAgent: userAgent,
	}

	_, err = d.Exec(
		"INSERT INTO sessions VALUES (?, ?, ?, ?, ?)",
		s.ID, s.Username, s.AuthToken, s.Deadline, s.UserAgent,
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to save session")
	}

	return &s, nil
}

func randToken() (string, error) {
	var token = make([]byte, 32)

	if _, err := rand.Read(token); err != nil {
		return "", errors.Wrap(err, "Failed to generate randomness")
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}
