package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Tx runs fn inside a write transaction, committing if it returns nil and
// rolling back if it does not.
//
// Every write that matters goes through here. docs/02-domain.md is blunt about
// why: an invoice number allocated for an order that then fails to commit is a
// gap in the series, and a gap is a compliance problem in most jurisdictions,
// not a bug to tidy up later.
//
// The rollback is deferred, so it also runs when fn panics. The panic then
// carries on: swallowing it would hide a bug, and leaving the transaction open
// would hold the single writer connection until the busy timeout expires on
// whoever tries to sell something next.
//
// fn's error is wrapped, not replaced, so a typed domain error still matches
// with errors.Is on the way out.
func (d *DB) Tx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := d.write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: beginning a transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// sql.ErrTxDone means the transaction is already finished, which is
		// the ordinary case when the context was cancelled underneath it.
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("db: rolling back: %w", rollbackErr))
		}
	}()

	if err := fn(tx); err != nil {
		return fmt.Errorf("db: in transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: committing: %w", err)
	}
	committed = true

	return nil
}
