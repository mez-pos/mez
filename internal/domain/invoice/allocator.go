package invoice

import (
	"context"
	"fmt"
	"time"
)

// Allocator hands out invoice numbers. The domain defines the contract; the
// database layer implements it, and proves it with the suite in
// invoicetest.Conformance.
//
// The contract:
//
//  1. The first number for a Key is 1.
//  2. Each call returns one more than the last number committed or already
//     allocated in the same transaction for that Key. Never more.
//  3. Keys are independent: a venue, series or year never shares a counter.
//  4. An Allocator is bound to the transaction that issues the invoice. If
//     that transaction rolls back, its numbers were never allocated and the
//     next transaction gets them again. That is what makes the series
//     gapless when a sale fails to commit.
//  5. Allocation is serialised per Key: two transactions can never be handed
//     the same number. In SQLite that is the single writer plus the counter
//     row in invoice_series; it is never MAX(n)+1 over issued invoices.
type Allocator interface {
	Next(ctx context.Context, key Key) (int64, error)
}

// Issued is one invoice number as allocated and written.
type Issued struct {
	Key       Key
	FY        FinancialYear
	Seq       int64
	InvoiceNo string
}

// Issue allocates and formats the next invoice number in a series, for a sale
// at a time and place. It is the one path from a sale to a number.
//
// Any error means the caller must roll back the transaction a is bound to.
// A number allocated but not written is otherwise a gap: rolling back is
// what returns it.
func Issue(ctx context.Context, a Allocator, s Series, venueID int64, at time.Time, loc *time.Location) (Issued, error) {
	fy, err := s.FY.YearOf(at, loc)
	if err != nil {
		return Issued{}, err
	}
	key := s.KeyFor(venueID, fy)
	if err := key.Validate(); err != nil {
		return Issued{}, err
	}
	seq, err := a.Next(ctx, key)
	if err != nil {
		return Issued{}, fmt.Errorf("allocating in series %q: %w", s.Code, err)
	}
	if seq < 1 {
		return Issued{}, fmt.Errorf("%w: %d for %+v", ErrInvalidSeq, seq, key)
	}
	no, err := s.Number(fy, seq)
	if err != nil {
		return Issued{}, err
	}
	return Issued{Key: key, FY: fy, Seq: seq, InvoiceNo: no}, nil
}
