package escpos

import (
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

// ZReport is the end-of-day summary a shift close prints. Phase 09 works out
// the figures; this only lays them out, so every amount here is given, never
// summed. An X-report is the same with Final false: a look mid-shift that
// closes nothing.
type ZReport struct {
	Final    bool
	Copy     bool // printed again on request, not with the close
	Venue    string
	Device   string
	Number   int64     // the Z count, never reset; 0 for an X-report
	From, To time.Time // the shift, in the venue's zone
	By       string    // who closed it

	// A correction names the report it corrects, and why (ADR-098).
	Corrects int64
	Reason   string
	// TradingDay is the day the shift is reported under, "2026-09-28".
	TradingDay string

	Orders       int64
	Gross        money.Amount // as priced, before discounts
	Discounts    money.Amount
	Taxable      money.Amount
	Taxes        []ZTax
	Total        money.Amount  // what guests owed
	NetOfCredits *money.Amount // Total less the credit notes; nil to leave out
	Categories   []ZLine
	Channels     []ZLine
	Staff        []ZLine
	Tenders      []ZTender
	Tips         money.Amount
	Refunds      ZCount
	CreditNotes  ZCount
	Voids        ZCount
	Removed      ZCount // rung up by mistake and taken off, no manager
	Comps        ZCount
	Discounted   ZCount // how many discounts, and what they took off
	FirstInvoice string
	LastInvoice  string
	Invoices     int64
	Reprints     int64
	Unbalanced   *money.Amount // payments less sales, when not zero
	// Carried is the orders still open at the close and what was still to
	// pay on them, which go on into the next shift; nil to leave out.
	Carried        *ZCount
	CashFloat      *money.Amount // the drawer's make-up; nil to leave out
	CashIn         money.Amount
	CashOut        money.Amount
	CashSales      money.Amount
	CashExpected   money.Amount
	CashCounted    *money.Amount // nil until the blind count is entered
	CashDifference *money.Amount // counted less expected, from phase 09
	CashReason     string        // why the drawer was over or short
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

// ZLine is one heading's line: a category, a channel, a staff member.
type ZLine struct {
	Name   string
	Count  int64
	Amount money.Amount
}

// Layout lays the report out for p and ends it with a full cut.
func (z ZReport) Layout(p Paper) ([]byte, error) {
	w := p.Columns
	b := New()
	z.header(b, w)

	section(b, "SALES", w)
	pair(b, "Orders", fmt.Sprint(z.Orders), w)
	total(b, "Gross", z.Gross, w)
	if z.Discounted.Count > 0 {
		total(b, fmt.Sprintf("Discounts (%d)", z.Discounted.Count), negate(z.Discounts), w)
	} else {
		total(b, "Discounts", negate(z.Discounts), w)
	}
	total(b, "Taxable value", z.Taxable, w)
	for _, t := range z.Taxes {
		total(b, t.Name, t.Amount, w)
	}
	b.Bold(true)
	total(b, "Total", z.Total, w)
	b.Bold(false)
	if z.NetOfCredits != nil {
		total(b, "Less credit notes", negate(z.CreditNotes.Amount), w)
		total(b, "Net sales", *z.NetOfCredits, w)
	}
	if z.Carried != nil {
		counted(b, "Open orders carried", *z.Carried, w)
	}
	lines(b, "BY CATEGORY", z.Categories, w)
	lines(b, "BY CHANNEL", z.Channels, w)
	lines(b, "BY STAFF", z.Staff, w)

	section(b, "PAYMENTS", w)
	for _, t := range z.Tenders {
		total(b, fmt.Sprintf("%s (%d)", methodWord(t.Method), t.Count), t.Amount, w)
	}
	total(b, "Tips, apart from sales", z.Tips, w)
	if z.Unbalanced != nil {
		total(b, "Paid for other shifts", *z.Unbalanced, w)
	}

	section(b, "EXCEPTIONS", w)
	counted(b, "Refunds", z.Refunds, w)
	if z.CreditNotes.Count > 0 {
		counted(b, "Credit notes", z.CreditNotes, w)
	}
	counted(b, "Voids", z.Voids, w)
	if z.Removed.Count > 0 {
		counted(b, "Removed in error", z.Removed, w)
	}
	counted(b, "Comps", z.Comps, w)
	pair(b, "Reprints", fmt.Sprint(z.Reprints), w)

	section(b, "INVOICES", w)
	pair(b, "Issued", fmt.Sprint(z.Invoices), w)
	if z.Invoices > 0 {
		pair(b, "First", z.FirstInvoice, w)
		pair(b, "Last", z.LastInvoice, w)
	}

	z.drawer(b, w)
	b.Line(rule('=', w)).Feed(3)
	return b.Cut(false).Bytes()
}

func (z ZReport) header(b *Builder, w int) {
	b.Align(Center).Bold(true).Size(2, 2)
	if z.Final {
		b.Line(fmt.Sprintf("Z-REPORT %d", z.Number))
	} else {
		b.Line("X-REPORT")
	}
	b.Size(1, 1).Line(Printable(z.Venue)).Bold(false)
	if !z.Final {
		b.Line("Not a close: nothing was reset")
	}
	if z.Copy {
		b.Bold(true).Line("COPY").Bold(false)
	}
	if z.Corrects > 0 {
		b.Bold(true).Line(fmt.Sprintf("CORRECTS Z-REPORT %d", z.Corrects)).Bold(false)
	}
	b.Align(Left).Feed(1)
	if z.Reason != "" {
		for _, s := range wrap(Printable(z.Reason), w) {
			b.Line(s)
		}
		b.Feed(1)
	}
	if z.TradingDay != "" {
		pair(b, "Trading day", z.TradingDay, w)
	}
	pair(b, "Till", z.Device, w)
	pair(b, "From", stamp(z.From), w)
	pair(b, "To", stamp(z.To), w)
	pair(b, "Closed by", z.By, w)
}

func (z ZReport) drawer(b *Builder, w int) {
	section(b, "CASH DRAWER", w)
	if z.CashFloat != nil {
		total(b, "Float", *z.CashFloat, w)
		total(b, "Cash in", z.CashIn, w)
		total(b, "Cash out", negate(z.CashOut), w)
		total(b, "Cash sales, net", z.CashSales, w)
	}
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
	if z.CashReason != "" {
		for _, s := range wrap("Why: "+Printable(z.CashReason), w) {
			b.Line(s)
		}
	}
}

func lines(b *Builder, name string, ls []ZLine, w int) {
	if len(ls) == 0 {
		return
	}
	section(b, name, w)
	for _, l := range ls {
		total(b, fmt.Sprintf("%s (%d)", Printable(l.Name), l.Count), l.Amount, w)
	}
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
