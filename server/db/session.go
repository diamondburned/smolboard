package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"time"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/pkg/errors"
)

// querysmolboard.Session searches for a session..
func (d *Transaction) querySession(token string) (*smolboard.Session, error) {
	var s smolboard.Session

	err := d.
		QueryRowx("SELECT * FROM sessions WHERE authtoken = ?", token).
		StructScan(&s)

	if err != nil {
		// Treat session not found errors as expired to make them the same as
		// actual expired (and deleted) tokens.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, smolboard.ErrSessionExpired
		}

		return nil, errors.Wrap(err, "Failed to scan session")
	}

	var now = time.Now()

	// If the token is expired, then (try to) delete it and return the expired
	// error.
	if now.UnixNano() > s.Deadline {
		return nil, smolboard.ErrSessionExpired
	}

	// Bump up the expiration time.
	now = now.Add(d.config.tokenLifespan)
	s.Deadline = now.UnixNano()

	_, err = d.Exec(
		"UPDATE sessions SET deadline = ? WHERE authtoken = ?",
		s.Deadline, s.AuthToken,
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to renew token")
	}

	return &s, nil
}

func (d *Transaction) cleanupSession(now int64) error {
	// Execute cleanup of expired sessions.
	_, err := d.Exec(
		"DELETE FROM sessions WHERE deadline < ?",
		time.Now().UnixNano(),
	)

	if err != nil {
		return errors.Wrap(err, "Faield to cleanup expired sessions")
	}

	return nil
}

func (d *Transaction) newSession(username, userAgent string) (*smolboard.Session, error) {
	t, err := randToken()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate a token")
	}

	var now = time.Now()

	s := &smolboard.Session{
		ID:        int64(sessionIDGen.Generate()),
		Username:  username,
		AuthToken: t,
		Deadline:  now.Add(d.config.tokenLifespan).UnixNano(),
		UserAgent: userAgent,
	}

	_, err = d.Exec(
		"INSERT INTO sessions VALUES (?, ?, ?, ?, ?)",
		s.ID, s.Username, s.AuthToken, s.Deadline, s.UserAgent,
	)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to save session")
	}

	// Execute cleanup of expired sessions.
	return s, d.cleanupSession(now.UnixNano())
}

// Signin creates a new session using the given username and password. The
// UserAgent will be used for listing sessions. This function returns an
// authenticate token.
func (d *Transaction) Signin(user, pass, UA string) (*smolboard.Session, error) {
	r := d.QueryRow("SELECT passhash FROM users WHERE username = ?", user)

	var passhash []byte
	if err := r.Scan(&passhash); err != nil {
		// Return an invalid password for a non-existent user.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, smolboard.ErrInvalidPassword
		}

		return nil, errors.Wrap(err, "Failed to scan for password")
	}

	if err := VerifyPassword(passhash, pass); err != nil {
		return nil, err
	}

	return d.newSession(user, UA)
}

func (d *Transaction) Signup(user, pass, token, UA string) (*smolboard.Session, error) {
	// Verify the token.
	if err := d.useToken(token); err != nil {
		return nil, err
	}

	if err := d.createUser(user, pass, smolboard.PermissionUser); err != nil {
		return nil, err
	}

	return d.newSession(user, UA)
}

func (d *Transaction) Signout() error {
	c, err := d.execChanged(
		"DELETE FROM sessions WHERE authtoken = ?",
		d.Session.AuthToken,
	)
	if err != nil {
		return errors.Wrap(err, "Failed to delete token")
	}
	if !c {
		return smolboard.ErrSessionNotFound
	}
	return err
}

func (d *Transaction) Sessions() ([]smolboard.Session, error) {
	r, err := d.Queryx("SELECT * FROM sessions WHERE username = ?", d.Session.Username)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query for sessions")
	}

	defer r.Close()

	var sessions []smolboard.Session

	for r.Next() {
		var s smolboard.Session

		if err := r.StructScan(&s); err != nil {
			return nil, errors.Wrap(err, "Failed to scan to a session")
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// DeleteSessionID deletes the person's own session ID.
func (d *Transaction) DeleteSessionID(id int64) error {
	// Ensure that we are deleting only this user's token.
	c, err := d.execChanged(
		"DELETE FROM sessions WHERE id = ? AND username = ?",
		id, d.Session.Username,
	)
	if err != nil {
		return errors.Wrap(err, "Failed to delete token with ID")
	}
	if !c {
		return smolboard.ErrSessionNotFound
	}
	return nil
}

func randToken() (string, error) {
	var token = make([]byte, 32)

	if _, err := rand.Read(token); err != nil {
		return "", errors.Wrap(err, "Failed to generate randomness")
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}
