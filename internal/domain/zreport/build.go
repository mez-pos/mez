package zreport

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// ErrInconsistent means a bill's lines do not add up to its total: the
// projection and the report disagree, and the report is not issued.
var ErrInconsistent = errors.New("zreport: the figures do not add up")

// Category is the menu category an item is reported under.
type Category struct {
	ID   int64
	Name string
}

// Uncategorised is where an item with no category is reported.
var Uncategorised = Category{ID: 0, Name: "Uncategorised"}

// Input is everything a report is built from.
type Input struct {
	ShiftID  int64
	Currency string
	Taxes    order.Taxes
	// Orders are the whole streams of the orders with any event in the
	// shift, each stamped with the shift it was recorded in.
	Orders [][]shift.StampedEvent
	// Categories are the menu's categories by item ID, as they stand when
	// the report is built.
	Categories map[int64]Category
}

// Build adds up a shift's orders into its report.
func Build(in Input) (Report, error) {
	const key = "shift"
	reports, err := BuildEach(in, []string{key}, func(se shift.StampedEvent) []string {
		if se.ShiftID == in.ShiftID {
			return []string{key}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	return reports[key], nil
}

// Keys names the reports an event is counted in: none, one or several.
type Keys func(shift.StampedEvent) []string

// BuildEach adds up the orders into one report per key, in one pass over
// them (the range reports, ADR-118). An event is counted, exactly as a
// Z-report counts it, in each report keys names for it. Every key in want
// has a report, an empty one if nothing was counted in it; a key keys
// names that is not in want is counted all the same. in.ShiftID is unused.
func BuildEach(in Input, want []string, keys Keys) (map[string]Report, error) {
	zero, err := money.New(0, in.Currency)
	if err != nil {
		return nil, fmt.Errorf("zreport: %w", err)
	}
	builders := make(map[string]*builder, len(want))
	builderOf := func(k string) *builder {
		b, ok := builders[k]
		if !ok {
			b = newBuilder(in, zero)
			builders[k] = b
		}
		return b
	}
	for _, k := range want {
		builderOf(k)
	}
	for _, events := range in.Orders {
		if err := walk(events, in.Taxes, keys, builderOf); err != nil {
			return nil, err
		}
	}
	out := make(map[string]Report, len(builders))
	for k, b := range builders {
		r, err := b.finish()
		if err != nil {
			return nil, err
		}
		out[k] = r
	}
	return out, nil
}

// walk goes through one order's stream, projecting as it goes, and counts
// each event in the reports keys names, against the state just before and
// after it.
func walk(events []shift.StampedEvent, taxes order.Taxes, keys Keys, builderOf func(string) *builder) error {
	var before order.State
	for _, se := range events {
		after, err := order.Resume(before, []order.Event{se.Event}, taxes)
		if err != nil {
			return fmt.Errorf("zreport: projecting: %w", err)
		}
		for _, k := range keys(se) {
			if err := builderOf(k).event(se.Event, before, after); err != nil {
				return fmt.Errorf("zreport: %s at seq %d: %w", se.Event.Type, se.Event.Seq, err)
			}
		}
		before = after
	}
	return nil
}

// event counts one of the shift's events.
func (b *builder) event(e order.Event, before, after order.State) error {
	switch p := e.Payload.(type) {
	case order.InvoiceIssued:
		return b.sale(p, after)
	case order.CreditNoteIssued:
		return b.credit(p)
	case order.InvoiceReprinted:
		b.r.Reprints++
	case order.LineVoided:
		return b.void(e, p, before)
	case order.LineComped:
		line, err := lineOf(before, p.LineID)
		if err != nil {
			return err
		}
		return b.tally(&b.r.Comps, line.Gross)
	case order.PaymentTendered:
		return b.tender(p)
	case order.PaymentVoided:
		t, err := tenderOf(before, p.PaymentID)
		if err != nil {
			return err
		}
		pay := b.payment(t.Method)
		b.add(&pay.Voided, t.Amount)
		b.sub(&b.r.Tips, nonZero(t.Tip, b.zero))
		return b.err
	case order.RefundIssued:
		t, err := tenderOf(before, p.AgainstPaymentID)
		if err != nil {
			return err
		}
		b.add(&b.payment(t.Method).Refunded, p.Amount)
		return b.tally(&b.r.Refunds, p.Amount)
	}
	return nil
}

func (b *builder) void(e order.Event, p order.LineVoided, before order.State) error {
	line, err := lineOf(before, p.LineID)
	if err != nil {
		return err
	}
	if p.ReasonCode != order.ReasonEnteredInError {
		return b.tally(&b.r.Voids, line.Gross)
	}
	t, ok := b.removed[e.ActorID]
	if !ok {
		t = &StaffTally{StaffID: e.ActorID, Tally: Tally{Amount: b.zero}}
		b.removed[e.ActorID] = t
	}
	return b.tally(&t.Tally, line.Gross)
}

func (b *builder) tender(p order.PaymentTendered) error {
	pay := b.payment(p.Method)
	pay.Count++
	b.add(&pay.Taken, p.Amount)
	b.add(&b.r.Tips, nonZero(p.Tip, b.zero))
	return b.err
}

// credit counts a credit note: sales reversed, and its tax off the tax lines.
func (b *builder) credit(p order.CreditNoteIssued) error {
	if err := b.tally(&b.r.CreditNotes, p.Total); err != nil {
		return err
	}
	for _, c := range p.Components {
		t := b.taxLine(c.Name, c.Rate)
		b.sub(&t.Taxable, c.Taxable)
		b.sub(&t.Amount, c.Amount)
	}
	return b.err
}

// sale counts a bill as it stood when its invoice was issued. Every
// heading is built from the same active lines, so each adds up to the
// same sales.
func (b *builder) sale(p order.InvoiceIssued, st order.State) error {
	b.invoices = append(b.invoices, p.InvoiceNo)
	bill := Group{Qty: 1, Gross: b.zero, Discounts: b.zero, Tax: b.zero, Total: b.zero}
	for _, l := range st.Lines {
		if l.Status == order.LineStatusActive {
			b.line(l, &bill)
		}
	}
	if b.err == nil && bill.Total != st.Totals.Total {
		return fmt.Errorf("%w: invoice %s's lines come to %s, its total is %s", ErrInconsistent, p.InvoiceNo, bill.Total, st.Totals.Total)
	}
	s := &b.r.Sales
	s.Bills++
	b.add(&s.Gross, bill.Gross)
	b.add(&s.Discounts, bill.Discounts)
	b.add(&s.Taxable, st.Totals.Taxable)
	b.add(&s.Tax, bill.Tax)
	b.add(&s.Total, bill.Total)
	b.add(&b.r.Discounts.Amount, bill.Discounts)
	staff := strconv.FormatInt(st.OpenedBy, 10)
	for _, g := range []*Group{
		b.group(b.channels, string(st.Channel), string(st.Channel)),
		b.group(b.staff, staff, staff),
	} {
		g.Qty++
		b.into(g, bill.Gross, bill.Discounts, bill.Tax, bill.Total)
	}
	for _, d := range st.Discounts {
		if !d.Value.IsZero() {
			b.r.Discounts.Count++
		}
	}
	for _, c := range st.TaxDetail.Components {
		t := b.taxLine(c.Name, c.Rate)
		b.add(&t.Taxable, c.Taxable)
		b.add(&t.Amount, c.Amount)
	}
	return b.err
}

// line counts an active line of an invoiced bill under its item and its
// category, and into the bill.
func (b *builder) line(l order.Line, bill *Group) {
	discount := b.zero
	b.add(&discount, l.Discount)
	b.add(&discount, l.OrderDiscount)
	cat, ok := b.in.Categories[l.ItemID]
	if !ok {
		cat = Uncategorised
	}
	item := strconv.FormatInt(l.ItemID, 10)
	for _, g := range []*Group{
		b.group(b.items, item, l.Name),
		b.group(b.categories, strconv.FormatInt(cat.ID, 10), cat.Name),
	} {
		g.Qty += l.Qty
		b.into(g, l.Gross, discount, l.Tax, l.Due)
	}
	b.into(bill, l.Gross, discount, l.Tax, l.Due)
}

func (b *builder) into(g *Group, gross, discount, tax, total money.Amount) {
	b.add(&g.Gross, gross)
	b.add(&g.Discounts, discount)
	b.add(&g.Tax, tax)
	b.add(&g.Total, total)
}

func lineOf(st order.State, id int64) (order.Line, error) {
	for _, l := range st.Lines {
		if l.ID == id {
			return l, nil
		}
	}
	return order.Line{}, fmt.Errorf("%w: no line %d", ErrInconsistent, id)
}

func tenderOf(st order.State, id int64) (order.Tender, error) {
	for _, t := range st.Tenders {
		if t.ID == id {
			return t, nil
		}
	}
	return order.Tender{}, fmt.Errorf("%w: no payment %d", ErrInconsistent, id)
}

// nonZero is a, or zero when a is the empty Amount an old tender carries
// for no tip.
func nonZero(a, zero money.Amount) money.Amount {
	if a == (money.Amount{}) {
		return zero
	}
	return a
}
