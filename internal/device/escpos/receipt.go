package escpos

import (
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// Receipt is a customer's copy of an invoice, from the invoice's own
// snapshots (ADR-078): nothing here is recomputed, only laid out.
//
// A Receipt with no InvoiceNo is a bill printed before payment. It has no
// number, because numbers come with the settling payment, and it says in
// capitals that it is not a tax invoice (under the GST composition scheme,
// not a bill of supply).
type Receipt struct {
	InvoiceNo string
	IssuedAt  time.Time // in the venue's zone; the layout prints it as given
	OrderRef  string    // the order's public id, for finding it again
	Table     int64     // 0 for a counter sale
	Prices    order.InvoicePrices
	Taxes     []tax.ComponentTotal // the rate-wise summary from the tax snapshot
	Tenders   []order.Tender       // voided ones are left off
	Copy      bool                 // a reprint: marked DUPLICATE, same number
	Template  Template             // what the venue adds, and which optional parts print
	Kick      bool                 // open the cash drawer as the receipt starts: cash was taken
}

// Layout lays the receipt out for p and ends it with a partial cut.
func (r Receipt) Layout(p Paper) ([]byte, error) {
	w := p.Columns
	b := New()
	if r.Kick {
		// First, so the drawer opens while the receipt prints, not after.
		b.KickDrawer(DrawerPin2, 2)
	}
	r.header(b, w)

	b.Line(rule('-', w))
	for _, l := range r.Prices.Lines {
		for _, s := range columns(fmt.Sprintf("%d x %s", l.Qty, l.Name), amountText(l.Gross, false), w) {
			b.Line(s)
		}
		if len(l.Modifiers) > 0 && !r.Template.HideModifiers {
			names := make([]string, len(l.Modifiers))
			for i, m := range l.Modifiers {
				names[i] = m.Name
			}
			indented(b, strings.Join(names, ", "), w)
		}
		if l.HSN != "" {
			indented(b, "HSN/SAC "+l.HSN, w)
		}
		if !l.Discount.IsZero() {
			total(b, "  Discount", negate(l.Discount), w)
		}
	}
	b.Line(rule('-', w))

	p2 := r.Prices
	if !p2.LineDiscounts.IsZero() || !p2.OrderDiscounts.IsZero() {
		total(b, "Items", p2.Gross, w)
		if !p2.LineDiscounts.IsZero() {
			total(b, "Line discounts", negate(p2.LineDiscounts), w)
		}
		if !p2.OrderDiscounts.IsZero() {
			total(b, "Order discount", negate(p2.OrderDiscounts), w)
		}
	}
	// Taxable value, each tax, then the total: how a tax invoice reads
	// under either pricing.
	if len(r.Taxes) > 0 {
		total(b, "Taxable value", p2.Taxable, w)
		for _, c := range r.Taxes {
			total(b, fmt.Sprintf("%s %s", c.Name, rateText(c.Rate)), c.Amount, w)
		}
	} else {
		total(b, "Subtotal", p2.Subtotal, w)
	}
	b.Line(rule('=', w))
	// The total at double height only: double width would halve the line and
	// push a large total's digits onto a second line.
	b.Bold(true).Size(1, 2)
	for _, s := range columns("TOTAL", amountText(p2.Total, true), w) {
		b.Line(s)
	}
	b.Size(1, 1).Bold(false)
	b.Line(rule('=', w))

	r.payments(b, w)
	r.footer(b, w)
	return b.Cut(true).Bytes()
}

func (r Receipt) header(b *Builder, w int) {
	s := r.Prices.Seller
	b.Align(Center).Bold(true).Size(1, 2)
	for _, l := range wrap(s.LegalName, w) {
		b.Line(l)
	}
	b.Size(1, 1).Bold(false)
	if s.Name != "" && s.Name != s.LegalName {
		b.Line(s.Name)
	}
	if s.Registration != "" {
		b.Line(registrationLabel(s.TaxProfile) + " " + s.Registration)
	}
	for _, l := range textLines(r.Template.Header, w) {
		b.Line(l)
	}
	b.Feed(1)

	b.Bold(true)
	switch {
	case r.InvoiceNo == "":
		b.Line("BILL - NOT A " + notA(s.TaxProfile))
	case r.Copy:
		b.Line(title(s.TaxProfile) + " - DUPLICATE")
	default:
		b.Line(title(s.TaxProfile))
	}
	b.Bold(false)
	// At the top of every bill of supply, in the rule's words (CGST Rule
	// 5(1)(g), ADR-401).
	if r.InvoiceNo != "" && s.TaxProfile == tax.CodeGSTComposition {
		for _, l := range wrap(tax.CompositionDeclaration, w) {
			b.Line(l)
		}
	}
	b.Align(Left)

	if r.InvoiceNo != "" {
		b.Line("No.   " + r.InvoiceNo)
	}
	if !r.IssuedAt.IsZero() {
		b.Line("Date  " + r.IssuedAt.Format("02/01/2006 15:04"))
	}
	if s.StateCode != "" {
		b.Line("Place of supply  " + s.StateCode)
	}
	ref := r.OrderRef
	if r.Template.HideOrderRef {
		ref = ""
	}
	if r.Table > 0 {
		ref = strings.TrimRight(fmt.Sprintf("Table %d  %s", r.Table, ref), " ")
	}
	if ref != "" {
		b.Line(ref)
	}
}

func (r Receipt) payments(b *Builder, w int) {
	for _, t := range r.Tenders {
		if t.Status == order.PaymentStatusVoided {
			continue
		}
		label := "Paid " + methodWord(t.Method)
		if t.Reference != "" {
			label += " " + t.Reference
		}
		total(b, label, t.Amount, w)
		if !t.Tip.IsZero() {
			total(b, "  Tip, apart from the bill", t.Tip, w)
		}
	}
}

func (r Receipt) footer(b *Builder, w int) {
	b.Feed(1).Align(Center)
	if r.InvoiceNo == "" {
		for _, l := range wrap("Pay at the counter to receive your "+strings.ToLower(notA(r.Prices.Seller.TaxProfile))+".", w) {
			b.Line(l)
		}
	} else if !r.Template.HideBarcode {
		// The number as a barcode, so a refund or a reprint can scan the
		// guest's copy instead of typing it.
		b.Barcode(Printable(r.InvoiceNo), 60)
	}
	for _, l := range textLines(r.Template.Footer, w) {
		b.Line(l)
	}
	b.Align(Left).Feed(2)
}

// title is what the document is called under the venue's tax profile, the
// same words as the on-screen preview.
func title(profile string) string {
	switch profile {
	case tax.CodeGST:
		return "TAX INVOICE"
	case tax.CodeGSTComposition:
		return "BILL OF SUPPLY"
	case tax.CodeVAT:
		return "VAT RECEIPT"
	}
	return "INVOICE"
}

// notA is what a bill printed before payment says it is not: the document
// that payment issues. Every profile but the composition scheme says a tax
// invoice, as it always has.
func notA(profile string) string {
	if profile == tax.CodeGSTComposition {
		return "BILL OF SUPPLY"
	}
	return "TAX INVOICE"
}

// registrationLabel is what the seller's tax number is called under its
// profile, on a receipt and a credit note alike.
func registrationLabel(profile string) string {
	switch {
	case tax.UnderGST(profile):
		return "GSTIN"
	case profile == tax.CodeVAT:
		return "VAT No."
	}
	return "Reg."
}

func methodWord(m order.PaymentMethod) string {
	switch m {
	case order.PaymentCash:
		return "Cash"
	case order.PaymentCard:
		return "Card"
	case order.PaymentUPI:
		return "UPI"
	}
	return "Other"
}

// total is a label and an amount on one line, the amount flush right.
func total(b *Builder, label string, a money.Amount, w int) {
	for _, s := range columns(label, amountText(a, false), w) {
		b.Line(s)
	}
}

// indented is secondary text under a line, two spaces in.
func indented(b *Builder, text string, w int) {
	for _, l := range wrap(text, w-2) {
		b.Line("  " + l)
	}
}

func negate(a money.Amount) money.Amount { return money.Amount{Minor: -a.Minor, Currency: a.Currency} }

// rateText is a tax.Rate, thousandths of a percent, as "2.5%": by string,
// never through a float.
func rateText(r tax.Rate) string {
	whole := int64(r) / 1000
	rest := strings.TrimRight(fmt.Sprintf("%03d", abs(int64(r)%1000)), "0")
	if rest == "" {
		return fmt.Sprintf("%d%%", whole)
	}
	return fmt.Sprintf("%d.%s%%", whole, rest)
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
