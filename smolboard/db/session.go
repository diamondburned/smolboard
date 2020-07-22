package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"time"

	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/jmoiron/sqlx"
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

// NewSession creates a new session.
func NewSession(username, userAgent string, ttl time.Duration) (*Session, error) {
	t, err := randToken()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate a token")
	}

	return &Session{
		ID:        int64(sessionIDGen.Generate()),
		Username:  username,
		AuthToken: t,
		Deadline:  time.Now().Add(ttl).UnixNano(),
		UserAgent: userAgent,
	}, nil
}

func (s *Session) insert(tx *sql.Tx) error {
	_, err := tx.Exec(
		"INSERT INTO sessions VALUES (?, ?, ?, ?, ?)",
		s.ID, s.Username, s.AuthToken, s.Deadline, s.UserAgent,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to save session")
	}

	return nil
}

// scan fills Session using s.AuthToken.
func (s *Session) scan(tx *sqlx.Tx, renewTTL time.Duration) error {
	err := tx.
		QueryRowx("SELECT * FROM sessions WHERE authtoken = ?", s.AuthToken).
		StructScan(s)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionNotFound
		}

		return errors.Wrap(err, "Failed to scan session")
	}

	var now = time.Now()

	// If the token is expired, then (try to) delete it and return the expired
	// error.
	if now.UnixNano() > s.Deadline {
		return ErrSessionExpired
	}

	// Bump up the expiration time.
	now = now.Add(renewTTL)
	s.Deadline = now.UnixNano()

	_, err = tx.Exec(
		"UPDATE sessions SET deadline = ? WHERE authtoken = ?",
		s.Deadline, s.AuthToken,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to renew token")
	}

	return err
}

// Signin creates a new session using the given username and password. The
// UserAgent will be used for listing sessions. This function returns an
// authenticate token.
func (d *Database) Signin(ctx context.Context, user, pass, UA string) (*Session, error) {
	t, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to begin transaction")
	}
	defer t.Rollback()

	r := t.QueryRow("SELECT passhash FROM users WHERE username = ?", user)

	var passhash []byte
	if err := r.Scan(&passhash); err != nil {
		// Return an invalid password for a non-existent user.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidPassword
		}

		return nil, errors.Wrap(err, "Failed to scan for password")
	}

	if err := VerifyPassword(passhash, pass); err != nil {
		return nil, err
	}

	s, err := NewSession(user, UA, d.Config.tokenLifespan)
	if err != nil {
		return nil, err
	}

	if err := s.insert(t); err != nil {
		return nil, err
	}

	return s, t.Commit()
}

func (d *Database) Signup(ctx context.Context, user, pass, token, UA string) (*Session, error) {
	t, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to begin transaction")
	}
	defer t.Rollback()

	// Verify the token.
	if err := useToken(t, token); err != nil {
		return nil, err
	}

	u, err := NewUser(user, pass, PermissionNormal)
	if err != nil {
		return nil, err
	}

	if err := u.insert(t); err != nil {
		return nil, err
	}

	s, err := NewSession(user, UA, d.Config.tokenLifespan)
	if err != nil {
		return nil, err
	}

	if err := s.insert(t); err != nil {
		return nil, err
	}

	return s, t.Commit()
}

func (d *Transaction) Signout() error {
	_, err := d.Exec("DELETE FROM sessions WHERE authtoken = ?", d.session.AuthToken)
	return err
}

func (d *Transaction) Session() Session {
	return *d.session
}

func (d *Transaction) Sessions() ([]Session, error) {
	r, err := d.Queryx("SELECT * FROM sessions WHERE username = ?", d.session.Username)
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

// DeleteSessionID deletes the person's own session ID.
func (d *Transaction) DeleteSessionID(id int64) error {
	// Ensure that we are deleting only this user's token.
	_, err := d.Exec(
		"DELETE FROM sessions WHERE id = ? AND username = ?",
		id, d.session.Username,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionNotFound
		}

		return errors.Wrap(err, "Failed to delete token")
	}

	return nil
}

func (d *Transaction) CleanupSessions(now int64) error {
	_, err := d.Exec("DELETE FROM sessions WHERE deadline < ?", now)
	return err
}

func randToken() (string, error) {
	var token = make([]byte, 32)

	if _, err := rand.Read(token); err != nil {
		return "", errors.Wrap(err, "Failed to generate randomness")
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}
