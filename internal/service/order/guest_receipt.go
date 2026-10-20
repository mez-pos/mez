package order

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// ErrReceiptNotFound means there is no receipt to give for what was asked: no
// such bill, one that was never at the table asked from, one not yet settled,
// or one too old to give out. They are one answer, so that asking tells a
// stranger nothing of which.
var ErrReceiptNotFound = errors.New("no receipt for that bill at this table")

// ReceiptLife is how long after its invoice a bill's receipt is given to a
// guest. Long enough for someone claiming a meal back at the end of the
// month; after it the venue has the invoice, and a guest asks the venue.
const ReceiptLife = 45 * 24 * time.Hour

// Receipt is a settled bill as a guest keeps it (phase 11, Task 5d, ADR-240):
// the invoice the counter would print, with the till's parts taken out.
//
// Everything in it but the payments is read from the invoice's own snapshot,
// as a reprint is (ADR-078): it is what was billed, whatever the menu or the
// venue's details have become since.
type Receipt struct {
	// Bill is the bill's own id: what a receipt is asked for by.
	Bill      string
	InvoiceNo string
	IssuedAt  time.Time
	TimeZone  string
	Seller    order.InvoiceSeller
	Lines     []order.InvoiceLine
	// Items is what the lines come to, as priced; Discounts everything
	// taken off them.
	Items     money.Amount
	Discounts money.Amount
	Taxes     []tax.ComponentTotal
	// TaxIncluded says the prices already hold the tax.
	TaxIncluded bool
	Total       money.Amount
	// Payments is what was paid and how, never by whom or with what card.
	Payments []ReceiptPayment
	// Credited is what credit notes have since taken off the total, and
	// Refunded what was given back.
	Credited money.Amount
	Refunded money.Amount
}

// ReceiptPayment is one payment on a receipt: a method and an amount.
type ReceiptPayment struct {
	Method order.PaymentMethod
	Amount money.Amount
}

// Receipt reads a settled bill for a guest, by the bill's own id and the
// table they ask from. The bill must have been at that table: a table's code
// reads its own bills and no other's.
//
// A bill has a receipt once it is invoiced, which is when the payment that
// settles it lands, whether or not staff have closed it yet.
func (s *Service) Receipt(ctx context.Context, venueID, tableID int64, bill string) (Receipt, error) {
	notFound := func(why string) (Receipt, error) {
		return Receipt{}, fmt.Errorf("%w: %q from table %d: %s", ErrReceiptNotFound, bill, tableID, why)
	}
	row, err := s.db.Queries().OrderByPublicID(ctx, bill)
	if isNoRows(err) || (err == nil && row.VenueID != venueID) {
		return notFound("no such bill")
	}
	if err != nil {
		return Receipt{}, fmt.Errorf("reading bill %q: %w", bill, err)
	}
	res, err := s.Order(ctx, venueID, row.ID)
	if err != nil {
		return Receipt{}, err
	}
	st := res.State
	if !slices.Contains(st.Tables(), tableID) {
		return notFound("it was not at that table")
	}
	if !st.Invoiced || len(st.Invoices) == 0 {
		return notFound("it is not settled")
	}
	inv, err := s.Invoice(ctx, venueID, row.ID, st.Invoices[len(st.Invoices)-1].Seq)
	if err != nil {
		return Receipt{}, err
	}
	if inv.Prices == nil {
		return notFound("its invoice kept no prices")
	}
	if s.clock.Now().Sub(inv.IssuedAt) > ReceiptLife {
		return notFound("it is too old")
	}
	discounts, err := inv.Prices.LineDiscounts.Add(inv.Prices.OrderDiscounts)
	if err != nil {
		return Receipt{}, fmt.Errorf("adding up the discounts of %s: %w", inv.InvoiceNo, err)
	}
	out := Receipt{
		Bill: res.PublicID, InvoiceNo: inv.InvoiceNo, IssuedAt: inv.IssuedAt, TimeZone: inv.TimeZone,
		Seller: inv.Prices.Seller, Lines: inv.Prices.Lines,
		Items: inv.Prices.Gross, Discounts: discounts,
		Taxes: inv.Tax.Components, TaxIncluded: inv.Tax.Pricing == tax.Inclusive,
		Total:    inv.Prices.Total,
		Credited: st.Totals.Credited, Refunded: st.Totals.Refunded,
	}
	for _, t := range inv.Tenders {
		// A voided payment was never the guest's; one refunded since was,
		// and Refunded says what came back.
		if t.Status == order.PaymentStatusVoided || t.Status == order.PaymentStatusPending {
			continue
		}
		out.Payments = append(out.Payments, ReceiptPayment{Method: t.Method, Amount: t.Amount})
	}
	return out, nil
}
