package db

import (
	"context"
	"database/sql"
	"sync"

	"github.com/diamondburned/smolboard/smolboard"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// Transaction is a concurrent transaction in SQLite. The transaction is only
// truly committed once Commit is actually called, which would then lock the
// database.
type Transaction struct {
	// use a .Conn instead to allow concurrent transactions.
	*sqlx.Conn
	ctx    context.Context
	config DBConfig

	closeMu sync.Mutex
	closed  bool
	isTx    bool

	// As we acquire an entire transaction, it is safe to store our own local
	// session state as long as we keep it up to date on our own calls.
	Session smolboard.Session
}

// BeginTx starts a new transaction belonging to the given session. If session
// is empty, then a transaction is not opened.
func BeginTx(ctx context.Context, db *Database, session string) (*Transaction, error) {
	conn, err := db.Connx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get conn")
	}

	tx := Transaction{
		Conn:   conn,
		ctx:    ctx,
		config: db.Config,
	}

	if session != "" {
		_, err := conn.ExecContext(ctx, "BEGIN DEFERRED")
		if err != nil {
			conn.Close()
			return nil, errors.Wrap(err, "failed to acquire concurrent tx")
		}

		s, err := tx.querySession(session)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		tx.isTx = true
		tx.Session = *s
	}

	return &tx, nil
}

// Commit commits the changes. It does not close the transaction.
func (tx *Transaction) Commit() error {
	tx.closeMu.Lock()
	defer tx.closeMu.Unlock()

	if tx.closed {
		return nil
	}
	tx.closed = true

	defer tx.Conn.Close()

	if tx.isTx {
		_, err := tx.Conn.ExecContext(tx.ctx, "COMMIT")
		if err != nil {
			return err
		}
	}

	return nil
}

// Rollback rolls back the transaction and closes it. This method must ALWAYS be
// called when done.
func (tx *Transaction) Rollback() error {
	tx.closeMu.Lock()
	defer tx.closeMu.Unlock()

	if tx.closed {
		return nil
	}
	tx.closed = true

	defer tx.Conn.Close()

	if tx.isTx {
		_, err := tx.Conn.ExecContext(tx.ctx, "ROLLBACK")
		if err != nil {
			return err
		}
	}

	return nil
}

// Exec calls ExecContext.
func (tx *Transaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	return tx.Conn.ExecContext(tx.ctx, query, args...)
}

// QueryRowx calls QueryRowxContext.
func (tx *Transaction) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	return tx.Conn.QueryRowxContext(tx.ctx, query, args...)
}

// Queryx calls QueryxContext.
func (tx *Transaction) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	return tx.Conn.QueryxContext(tx.ctx, query, args...)
}

// QueryRow calls QueryRowContext.
func (tx *Transaction) QueryRow(query string, args ...interface{}) *sql.Row {
	return tx.Conn.QueryRowContext(tx.ctx, query, args...)
}

// Query calls QueryContext.
func (tx *Transaction) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Conn.QueryContext(tx.ctx, query, args...)
}
