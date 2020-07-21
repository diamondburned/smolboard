package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"

	"github.com/diamondburned/smolboard/utils/httperr"
	"github.com/pkg/errors"
)

type Token struct {
	Token     string `db:"token"`
	Remaining int    `db:"remaining"`
}

var ErrUnknownToken = httperr.New(401, "unknown token")

func (d *Database) CreateToken(uses int) (*Token, error) {
	// Generate a short random string.
	var r = make([]byte, 16)
	if _, err := rand.Read(r); err != nil {
		return nil, errors.Wrap(err, "Failed to generate randomness")
	}

	t := Token{
		Token:     base64.RawURLEncoding.EncodeToString(r),
		Remaining: uses,
	}

	_, err := d.Exec("INSERT INTO tokens VALUES (?, ?)", t.Token, t.Remaining)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to insert tokens")
	}

	return &t, nil
}

// UseToken returns an error if a token is not found, otherwise it decrements 1
// from the remaining key and return nil.
func (d *Database) UseToken(token string) error {
	// We need to start a transaction to avoid data race.
	tx, err := d.Beginx()
	if err != nil {
		return errors.Wrap(err, "Failed to start a transaction")
	}
	defer tx.Rollback()

	// See if we have the token.

	var remaining int

	err = d.
		QueryRow("SELECT remaining FROM tokens WHERE token = ?", token).
		Scan(&remaining)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnknownToken
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

	return tx.Commit()
}
