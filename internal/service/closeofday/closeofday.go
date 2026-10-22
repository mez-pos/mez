// Package closeofday reads what the owner's close-of-day message says
// (ADR-360) out of the Z-reports a venue has issued: thirteen fixed lines'
// worth of figures, every one taken from the report's stored body, which
// is what the slip printed, so the message and the slip cannot disagree.
// Nothing is recomputed here.
//
// mez Cloud reads it over a replica of the venue's backup (ADR-362). The
// cursor is the Z-report's row id: a reader keeps the last it saw, and
// asks for what came after.
package closeofday

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

// DefaultLimit is how many reports Since reads when asked for no number.
const DefaultLimit = 100

// Tally is how many of something, and what they came to, in minor units.
type Tally struct {
	Count  int64
	Amount int64
}

// Close is one Z-report's figures for the message. Every amount is in
// minor units of Currency.
type Close struct {
	// ID is the report's row, the reader's cursor. Number is the Z-report
	// number printed on the slip; Corrects is the number this one corrects,
	// or 0 for an original, and Reason why.
	ID       int64
	Number   int64
	Corrects int64
	Reason   string
	// GeneratedAt is when the report was issued, UTC.
	GeneratedAt time.Time

	// The shift: its public id, trading day, when it opened and closed, and
	// who closed it, by name as the report named them. Empty when the
	// report did not name them.
	ShiftID    string
	TradingDay string
	OpenedAt   time.Time
	ClosedAt   time.Time
	ClosedBy   string

	Currency string

	// Sales as invoiced: what the guests owed, the bills and the covers.
	// Difference is payments less sales, zero on a day whose bills were
	// all paid and invoiced within it (zreport.Balance).
	SalesTotal int64
	Bills      int64
	Covers     int64
	Difference int64

	// Each method's net: taken less voided and refunded. Other is every
	// method that is not cash, UPI or card.
	Cash, UPI, Card, Other int64

	// The drawer: the closing count and how it reconciled, over positive
	// and short negative.
	Counted  int64
	Variance int64

	Voids       Tally
	Comps       Tally
	Discounts   Tally
	CreditNotes Tally
	Refunds     Tally

	// Tips owed to staff; never in sales or payments.
	Tips int64

	// The invoices issued in the shift: how many, and the first and last
	// numbers.
	Invoices     int64
	FirstInvoice string
	LastInvoice  string
}

// Since is the venue's closes after the report with id afterID, oldest
// first, at most limit of them. Zero afterID is from the beginning; a
// limit of zero or less is DefaultLimit. A stored body that cannot be read
// is an error naming the report: it is the slip's record, and a message
// made up around it would be wrong.
func Since(ctx context.Context, q *gen.Queries, venueID, afterID, limit int64) ([]Close, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	rows, err := q.ZReportsAfter(ctx, gen.ZReportsAfterParams{VenueID: venueID, AfterID: afterID, PageSize: limit})
	if err != nil {
		return nil, fmt.Errorf("closeofday: reading the Z-reports of venue %d: %w", venueID, err)
	}
	out := make([]Close, 0, len(rows))
	for _, row := range rows {
		var doc svcshift.ZDocument
		if err := json.Unmarshal([]byte(row.Body), &doc); err != nil {
			return nil, fmt.Errorf("closeofday: Z-report %d's body cannot be read: %w", row.Number, err)
		}
		out = append(out, closeOf(row, doc))
	}
	return out, nil
}

// closeOf is the message's figures from one stored report.
func closeOf(row gen.ZReportsAfterRow, doc svcshift.ZDocument) Close {
	rep := doc.Report
	c := Close{
		ID: row.ID, Number: row.Number, Corrects: doc.Corrects, Reason: doc.Reason, GeneratedAt: row.GeneratedAt.UTC(),
		ShiftID: doc.Shift.PublicID, TradingDay: doc.Shift.TradingDay,
		OpenedAt: doc.Shift.OpenedAt.UTC(), ClosedAt: doc.Shift.ClosedAt.UTC(),
		ClosedBy:   doc.Names[doc.Shift.ClosedBy],
		Currency:   rep.Currency,
		SalesTotal: rep.Sales.Total.Minor, Bills: rep.Sales.Bills, Covers: rep.Sales.Covers,
		Difference: rep.Balance.Difference.Minor,
		Counted:    doc.Drawer.Counted.Minor, Variance: doc.Drawer.Variance.Minor,
		Voids:       tallyOf(rep.Voids.Count, rep.Voids.Amount.Minor),
		Comps:       tallyOf(rep.Comps.Count, rep.Comps.Amount.Minor),
		Discounts:   tallyOf(rep.Discounts.Count, rep.Discounts.Amount.Minor),
		CreditNotes: tallyOf(rep.CreditNotes.Count, rep.CreditNotes.Amount.Minor),
		Refunds:     tallyOf(rep.Refunds.Count, rep.Refunds.Amount.Minor),
		Tips:        rep.Tips.Minor,
		Invoices:    rep.Invoices.Count, FirstInvoice: rep.Invoices.First, LastInvoice: rep.Invoices.Last,
	}
	for _, p := range rep.Payments {
		switch p.Method {
		case order.PaymentCash:
			c.Cash += p.Net.Minor
		case order.PaymentUPI:
			c.UPI += p.Net.Minor
		case order.PaymentCard:
			c.Card += p.Net.Minor
		default:
			c.Other += p.Net.Minor
		}
	}
	return c
}

func tallyOf(count, amount int64) Tally { return Tally{Count: count, Amount: amount} }
