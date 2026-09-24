package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// ErrInvoiceNotIssued means an invoice was asked for that the order does not
// have: a number from another order, or none issued yet.
var ErrInvoiceNotIssued = errors.New("no such invoice on this order")

// pricesOf is the price snapshot for an invoice issued now: the venue as it
// is, and every active line as billed. Voided and comped lines are left
// out; they are on the timeline, not on the bill.
func pricesOf(st order.State, seller order.InvoiceSeller) *order.InvoicePrices {
	p := &order.InvoicePrices{
		Seller: seller, Lines: []order.InvoiceLine{},
		Gross: st.Totals.Gross, LineDiscounts: st.Totals.LineDiscounts, OrderDiscounts: st.Totals.OrderDiscounts,
		Subtotal: st.Totals.Subtotal, Taxable: st.Totals.Taxable, Tax: st.Totals.Tax, Total: st.Totals.Total,
	}
	for _, l := range st.Lines {
		if l.Status != order.LineStatusActive {
			continue
		}
		p.Lines = append(p.Lines, order.InvoiceLine{
			LineID: l.ID, Name: l.Name, Qty: l.Qty, UnitPrice: l.UnitPrice, Modifiers: l.Modifiers, HSN: l.HSN,
			Gross: l.Gross, Discount: l.Discount, OrderDiscount: l.OrderDiscount, Tax: l.Tax, Due: l.Due,
		})
	}
	return p
}

// IssuedInvoice is an invoice as it was issued, read back from its event:
// what a preview shows and a reprint prints. Nothing in it is recomputed.
type IssuedInvoice struct {
	Seq       int64
	InvoiceNo string
	Series    string
	IssuedAt  time.Time
	TimeZone  string
	Tax       tax.TaxResult
	Prices    *order.InvoicePrices // nil for invoices issued before prices were kept
	Reprints  int
	Tenders   []order.Tender // the payments on the order now, voided ones included
}

// Invoice reads one of an order's invoices by the seq of its
// invoice.issued: an invoice number holds slashes, which a path cannot.
func (s *Service) Invoice(ctx context.Context, venueID, orderID, seq int64) (IssuedInvoice, error) {
	h, err := s.OrderHistory(ctx, venueID, orderID)
	if err != nil {
		return IssuedInvoice{}, err
	}
	res, err := s.Order(ctx, venueID, orderID)
	if err != nil {
		return IssuedInvoice{}, err
	}
	for _, e := range h.Events {
		issued, ok := e.Payload.(order.InvoiceIssued)
		if !ok || e.Seq != seq {
			continue
		}
		out := IssuedInvoice{
			Seq: e.Seq, InvoiceNo: issued.InvoiceNo, Series: issued.Series, IssuedAt: e.At.UTC(),
			TimeZone: h.TimeZone, Prices: issued.Prices, Tenders: res.State.Tenders,
		}
		if err := json.Unmarshal(issued.TaxSnapshot, &out.Tax); err != nil {
			return IssuedInvoice{}, fmt.Errorf("reading the tax snapshot of %s: %w", issued.InvoiceNo, err)
		}
		for _, inv := range res.State.Invoices {
			if inv.InvoiceNo == issued.InvoiceNo {
				out.Reprints = inv.Reprints
			}
		}
		return out, nil
	}
	return IssuedInvoice{}, fmt.Errorf("%w: seq %d", ErrInvoiceNotIssued, seq)
}

// ReprintInvoice asks for a copy of an issued invoice, named by the seq of
// its invoice.issued. The caller checks the authoriser's permission.
type ReprintInvoice struct {
	InvoiceSeq   int64  `json:"invoice_seq"`
	Reason       string `json:"reason"`
	AuthorisedBy int64  `json:"authorised_by"`
}

// Reprint records a copy of an invoice. It allocates nothing: the copy
// carries the original number, and printing it is the print queue's job
// (phase 07, Task 5).
func (s *Service) Reprint(ctx context.Context, meta Meta, orderID int64, cmd ReprintInvoice) (Result, error) {
	return s.run(ctx, meta, orderID, "reprint_invoice", cmd, func(_ context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		for _, inv := range tx.State.Invoices {
			if inv.Seq == cmd.InvoiceSeq {
				return []order.Payload{order.InvoiceReprinted{
					InvoiceNo: inv.InvoiceNo, Reason: cmd.Reason, AuthorisedBy: cmd.AuthorisedBy,
				}}, nil
			}
		}
		return nil, fmt.Errorf("%w: seq %d", order.ErrInvoiceNotFound, cmd.InvoiceSeq)
	})
}
