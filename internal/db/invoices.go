package db

import (
	"context"
	"fmt"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/invoice"
)

// InvoiceAllocator is the database's invoice.Allocator: a counter row per
// venue, series and financial year in invoice_series, advanced by one upsert
// inside the issuing transaction. invoicetest.Conformance holds it to the
// contract.
//
// It is bound to the transaction its queries were made from. That binding is
// the contract's fourth rule: roll the transaction back and the number was
// never handed out. Built from the read pool it cannot allocate at all, which
// is the right failure.
type InvoiceAllocator struct {
	q *gen.Queries
}

// NewInvoiceAllocator binds an allocator to q, which must be bound to the
// write transaction issuing the invoice.
func NewInvoiceAllocator(q *gen.Queries) InvoiceAllocator {
	return InvoiceAllocator{q: q}
}

// Next allocates the next number for key.
//
// Serialisation is SQLite's: every write transaction holds the single write
// lock from its first statement (_txlock=immediate), so no two transactions
// ever read the same counter. The upsert reads and advances it in one
// statement, so there is no window between the two even within one.
func (a InvoiceAllocator) Next(ctx context.Context, key invoice.Key) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := key.Validate(); err != nil {
		return 0, err
	}
	seq, err := a.q.NextInvoiceSeq(ctx, gen.NextInvoiceSeqParams{
		VenueID: key.VenueID,
		Series:  key.Series,
		FyStart: int64(key.FYStart),
	})
	if err != nil {
		return 0, fmt.Errorf("db: allocating in %+v: %w", key, err)
	}
	return seq, nil
}

// recordInvoice writes an issued number to the register. The schema refuses
// a number the counter has not reached, and one already written.
func recordInvoice(ctx context.Context, q *gen.Queries, venueID, orderID int64, in invoice.Issued, at time.Time) error {
	if in.Key.VenueID != venueID {
		return fmt.Errorf("db: invoice %s was allocated for venue %d, the order is at venue %d", in.InvoiceNo, in.Key.VenueID, venueID)
	}
	_, err := q.CreateInvoice(ctx, gen.CreateInvoiceParams{
		VenueID:   venueID,
		OrderID:   orderID,
		Series:    in.Key.Series,
		FyStart:   int64(in.Key.FYStart),
		Seq:       in.Seq,
		InvoiceNo: in.InvoiceNo,
		IssuedAt:  at.UTC(),
	})
	if err != nil {
		return fmt.Errorf("db: recording invoice %s: %w", in.InvoiceNo, err)
	}
	return nil
}
