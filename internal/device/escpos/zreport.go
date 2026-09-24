package escpos

import (
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// ZReport is the end-of-day summary a shift close prints. Phase 09 works out
// the figures; this only lays them out, so every amount here is given, never
// summed. An X-report is the same with Final false: a look mid-shift that
// closes nothing.
type ZReport struct {
	Final    bool
	Venue    string
	Device   string
	Number   int64     // the Z count, never reset; 0 for an X-report
	From, To time.Time // the shift, in the venue's zone
	By       string    // who closed it

	Orders         int64
	Gross          money.Amount // as priced, before discounts
	Discounts      money.Amount
	Taxable        money.Amount
	Taxes          []ZTax
	Total          money.Amount // what guests owed
	Tenders        []ZTender
	Tips           money.Amount
	Refunds        ZCount
	Voids          ZCount
	Comps          ZCount
	FirstInvoice   string
	LastInvoice    string
	Invoices       int64
	Reprints       int64
	CashExpected   money.Amount
	CashCounted    *money.Amount // nil until the blind count is entered
	CashDifference *money.Amount // counted less expected, from phase 09
}

// ZTax is one tax component over the shift.
type ZTax struct {
	Name   string
	Amount money.Amount
}

// ZTender is one payment method over the shift.
type ZTender struct {
	Method order.PaymentMethod
	Count  int64
	Amount money.Amount
}

// ZCount is how many of something, and what they came to.
type ZCount struct {
	Count  int64
	Amount money.Amount
}

// Layout lays the report out for p and ends it with a full cut.
func (z ZReport) Layout(p Paper) ([]byte, error) {
	w := p.Columns
	b := New().Align(Center).Bold(true).Size(2, 2)
	if z.Final {
		b.Line(fmt.Sprintf("Z-REPORT %d", z.Number))
	} else {
		b.Line("X-REPORT")
	}
	b.Size(1, 1).Line(Printable(z.Venue)).Bold(false)
	if !z.Final {
		b.Line("Not a close: nothing was reset")
	}
	b.Align(Left).Feed(1)
	pair(b, "Till", z.Device, w)
	pair(b, "From", stamp(z.From), w)
	pair(b, "To", stamp(z.To), w)
	pair(b, "Closed by", z.By, w)

	section(b, "SALES", w)
	pair(b, "Orders", fmt.Sprint(z.Orders), w)
	total(b, "Gross", z.Gross, w)
	total(b, "Discounts", negate(z.Discounts), w)
	total(b, "Taxable value", z.Taxable, w)
	for _, t := range z.Taxes {
		total(b, t.Name, t.Amount, w)
	}
	b.Bold(true)
	total(b, "Total", z.Total, w)
	b.Bold(false)

	section(b, "PAYMENTS", w)
	for _, t := range z.Tenders {
		total(b, fmt.Sprintf("%s (%d)", methodWord(t.Method), t.Count), t.Amount, w)
	}
	total(b, "Tips, apart from sales", z.Tips, w)

	section(b, "EXCEPTIONS", w)
	counted(b, "Refunds", z.Refunds, w)
	counted(b, "Voids", z.Voids, w)
	counted(b, "Comps", z.Comps, w)
	pair(b, "Reprints", fmt.Sprint(z.Reprints), w)

	section(b, "INVOICES", w)
	pair(b, "Issued", fmt.Sprint(z.Invoices), w)
	if z.Invoices > 0 {
		pair(b, "First", z.FirstInvoice, w)
		pair(b, "Last", z.LastInvoice, w)
	}

	section(b, "CASH DRAWER", w)
	total(b, "Expected", z.CashExpected, w)
	if z.CashCounted != nil {
		total(b, "Counted", *z.CashCounted, w)
	} else {
		pair(b, "Counted", "not yet", w)
	}
	if z.CashDifference != nil {
		b.Bold(true)
		total(b, "Difference", *z.CashDifference, w)
		b.Bold(false)
	}
	b.Line(rule('=', w)).Feed(3)
	return b.Cut(false).Bytes()
}

func section(b *Builder, name string, w int) {
	b.Line(rule('-', w)).Bold(true).Line(name).Bold(false)
}

func pair(b *Builder, label, value string, w int) {
	for _, s := range columns(label, value, w) {
		b.Line(s)
	}
}

func counted(b *Builder, label string, c ZCount, w int) {
	total(b, fmt.Sprintf("%s (%d)", label, c.Count), c.Amount, w)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006 15:04")
}
