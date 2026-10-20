package escpos

import (
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// CreditNote is the guest's copy of a credit note (ADR-083), laid out from
// the note's own snapshot: what was credited against which invoice, the
// tax on it, and where the money went back.
type CreditNote struct {
	CreditNo         string
	AgainstInvoiceNo string
	IssuedAt         time.Time // in the venue's zone
	OrderRef         string
	Seller           order.InvoiceSeller
	Reason           string
	Lines            []order.CreditLine
	Taxable          money.Amount
	Tax              money.Amount
	Total            money.Amount
	Components       []tax.ComponentTotal
	Refunds          []CreditRefund
	Kick             bool // cash went back: open the drawer as it prints
	// Header and Footer are the venue's own lines (Template), as on a receipt:
	// the barcode and the optional parts of a receipt are not theirs to hide
	// here, a credit note having no choices or order reference of its own.
	Header []string
	Footer []string
}

// CreditRefund is money back against one payment.
type CreditRefund struct {
	Method    order.PaymentMethod
	Reference string
	Amount    money.Amount
}

// Layout lays the credit note out for p and ends it with a partial cut.
func (c CreditNote) Layout(p Paper) ([]byte, error) {
	w := p.Columns
	b := New()
	if c.Kick {
		b.KickDrawer(DrawerPin2, 2)
	}
	b.Align(Center).Bold(true).Size(1, 2)
	for _, l := range wrap(c.Seller.LegalName, w) {
		b.Line(l)
	}
	b.Size(1, 1).Bold(false)
	if c.Seller.Name != "" && c.Seller.Name != c.Seller.LegalName {
		b.Line(c.Seller.Name)
	}
	if c.Seller.Registration != "" {
		label := "Reg."
		switch c.Seller.TaxProfile {
		case "gst_in":
			label = "GSTIN"
		case "vat_uk":
			label = "VAT No."
		}
		b.Line(label + " " + c.Seller.Registration)
	}
	for _, l := range textLines(c.Header, w) {
		b.Line(l)
	}
	b.Feed(1).Bold(true).Size(1, 2).Line("CREDIT NOTE").Size(1, 1).Bold(false).Align(Left)
	b.Line("No.   " + c.CreditNo)
	b.Line("Against invoice " + c.AgainstInvoiceNo)
	if !c.IssuedAt.IsZero() {
		b.Line("Date  " + c.IssuedAt.Format("02/01/2006 15:04"))
	}
	if c.Seller.StateCode != "" {
		b.Line("Place of supply  " + c.Seller.StateCode)
	}
	if c.OrderRef != "" {
		b.Line(c.OrderRef)
	}
	for _, l := range wrap("Reason: "+c.Reason, w) {
		b.Line(l)
	}

	b.Line(rule('-', w))
	for _, l := range c.Lines {
		// Whole units say how many; a share of an amount says it is part.
		label := l.Name + " (part)"
		if l.Qty > 0 {
			label = fmt.Sprintf("%d x %s", l.Qty, l.Name)
		}
		for _, s := range columns(label, amountText(l.Amount, false), w) {
			b.Line(s)
		}
	}
	b.Line(rule('-', w))
	if len(c.Components) > 0 {
		total(b, "Taxable value", c.Taxable, w)
		for _, comp := range c.Components {
			total(b, fmt.Sprintf("%s %s", comp.Name, rateText(comp.Rate)), comp.Amount, w)
		}
	}
	b.Line(rule('=', w)).Bold(true).Size(1, 2)
	for _, s := range columns("CREDIT", amountText(c.Total, true), w) {
		b.Line(s)
	}
	b.Size(1, 1).Bold(false).Line(rule('=', w))
	for _, r := range c.Refunds {
		label := "Refunded to " + methodWord(r.Method)
		if r.Reference != "" {
			label += " " + r.Reference
		}
		total(b, label, r.Amount, w)
	}
	b.Feed(1).Align(Center).Barcode(Printable(c.CreditNo), 60)
	for _, l := range textLines(c.Footer, w) {
		b.Line(l)
	}
	b.Align(Left).Feed(2)
	return b.Cut(true).Bytes()
}
