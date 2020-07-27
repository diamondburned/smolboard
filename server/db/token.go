package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

func (d *Transaction) ListTokens() ([]smolboard.Token, error) {
	// Get the permission.
	p, err := d.Permission()
	if err != nil {
		return nil, err
	}

	// Exit if the user isn't at least an administrator.
	if p < smolboard.PermissionAdministrator {
		return nil, smolboard.ErrActionNotPermitted
	}

	var q *sqlx.Rows
	// Allow only the owner to see all tokens. Administrators can only see
	// their own tokens.
	if p == smolboard.PermissionOwner {
		q, err = d.Queryx("SELECT * FROM tokens")
	} else {
		q, err = d.Queryx(
			"SELECT * FROM tokens WHERE creator = ?",
			d.Session.Username,
		)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to query tokens")
	}

	defer q.Close()

	var tokens []smolboard.Token

	for q.Next() {
		var token smolboard.Token

		if err := q.StructScan(&token); err != nil {
			return nil, errors.Wrap(err, "Failed to scan token")
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (d *Transaction) DeleteToken(token string) error {
	r := d.QueryRow("SELECT creator FROM tokens WHERE token = ?", token)

	var creator string
	if err := r.Scan(&creator); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return smolboard.ErrUnknownToken
		}

		return errors.Wrap(err, "Failed to scan token's creator")
	}

	// A user can always revoke their own token, but they should at least be
	// an admin.
	if err := d.HasPermOverUser(smolboard.PermissionAdministrator, creator); err != nil {
		return err
	}

	_, err := d.Exec("DELETE FROM tokens WHERE token = ?", token)
	if err != nil {
		return errors.Wrap(err, "Failed to delete token")
	}

	return nil
}

func (d *Transaction) CreateToken(uses int) (*smolboard.Token, error) {
	// Is the user going over the max use limit? If yes, disallow it.
	if uses > d.config.MaxTokenUses {
		return nil, smolboard.ErrOverUseLimit
	}

	// Check the minimum required permission to make the token.
	var perm = smolboard.PermissionAdministrator
	// Is the user requesting for an unlimited use token?
	if uses == -1 {
		perm = smolboard.PermissionOwner
	}

	// Check permission.
	if err := d.HasPermission(perm, true); err != nil {
		return nil, err
	}

	// Generate a short random string.
	var r = make([]byte, 16)
	if _, err := rand.Read(r); err != nil {
		return nil, errors.Wrap(err, "Failed to generate randomness")
	}

	t := smolboard.Token{
		Token:     base64.RawURLEncoding.EncodeToString(r),
		Creator:   d.Session.Username,
		Remaining: uses,
	}

	_, err := d.Exec(
		"INSERT INTO tokens VALUES (?, ?, ?)",
		t.Token, d.Session.Username, t.Remaining)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to insert tokens")
	}

	return &t, nil
}

// useToken returns an error if a token is not found, otherwise it decrements 1
// from the remaining key and return nil.
func (d *Transaction) useToken(token string) error {
	// See if we have the token.
	var remaining int

	err := d.
		QueryRow("SELECT remaining FROM tokens WHERE token = ?", token).
		Scan(&remaining)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return smolboard.ErrUnknownToken
		}

		return errors.Wrap(err, "Failed to get token")
	}

	// If the token is an infinite use one, then we don't need to subtract.
	if remaining == -1 {
		// No need to commit, as we have not done any changes.
		return nil
	}

	// Consume SQL style. This query subtracts 1 from the current token then
	// deletes all tokens with remaining equals to 0.
	_, err = d.Exec(`
		UPDATE tokens SET remaining = remaining - 1 WHERE token = ?;
		DELETE FROM tokens WHERE remaining == 0`,
		token,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to clean up tokens")
	}

	return nil
}
