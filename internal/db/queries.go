package db

import (
	"context"
	"database/sql"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
)

// Queries is the generated query set bound to the read pool.
//
// It cannot write: the pool is opened mode=ro, so a statement that tries fails
// in SQLite rather than racing the writer. Reach for WriteTx to change
// anything.
func (d *DB) Queries() *gen.Queries {
	return gen.New(d.read)
}

// WriteTx runs fn inside a write transaction with the generated query set
// bound to it.
//
// This is how every write happens. The transaction is the point: an invoice
// number allocated for an order that then fails to commit is a gap in the
// series, and docs/02-domain.md is blunt about what a gap costs.
//
//	err := database.WriteTx(ctx, func(q *gen.Queries) error {
//	    venue, err := q.CreateVenue(ctx, params)
//	    ...
//	})
//
// Use Tx directly when a write needs SQL that sqlc does not generate, and
// Queries for reads.
func (d *DB) WriteTx(ctx context.Context, fn func(*gen.Queries) error) error {
	return d.Tx(ctx, func(tx *sql.Tx) error {
		return fn(gen.New(tx))
	})
}
