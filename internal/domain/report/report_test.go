package report

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/domain/tax"
	"github.com/mez-pos/mez/internal/domain/zreport"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

var kolkata = mustZone("Asia/Kolkata")

func mustZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// ist is a wall-clock time in Kolkata, as UTC, the way events carry it.
func ist(month time.Month, day, hour, min int) time.Time {
	return time.Date(2026, month, day, hour, min, 0, 0, kolkata).UTC()
}

var gstCafe = order.Taxes{Profile: tax.GST{}, Venue: tax.VenueContext{
	Currency: "INR", Pricing: tax.Exclusive, GSTIN: "29ABCDE1234F1Z5", StateCode: "29",
}}

const gst5 = tax.Rate(5000)

// bill is one order's stream, each event at its own time. Shifts do not
// matter to a range report, so every event is stamped shift 1.
type bill struct {
	t      *testing.T
	events []shift.StampedEvent
}

func newBill(t *testing.T, when time.Time, channel order.Channel, openedBy int64) *bill {
	b := &bill{t: t}
	b.add(when, order.OrderOpened{Channel: channel, Currency: "INR", OpenedBy: openedBy})
	return b
}

func (b *bill) add(when time.Time, p order.Payload) int64 {
	seq := int64(len(b.events) + 1)
	b.events = append(b.events, shift.StampedEvent{Event: order.NewEvent(seq, when, 3, 1, p), ShiftID: 1})
	return seq
}

func (b *bill) line(when time.Time, item int64, name string, qty, price int64) int64 {
	return b.add(when, order.LineAdded{ItemID: item, Name: name, Qty: qty, UnitPrice: inr(price), Station: "bar", TaxRate: gst5, HSN: "996331"})
}

func (b *bill) invoice(when time.Time, no string) {
	b.add(when, order.InvoiceIssued{InvoiceNo: no, Series: "INV", TaxSnapshot: json.RawMessage(`{}`)})
}

// credit credits some units of a line, planned as the till plans one.
func (b *bill) credit(when time.Time, lineID, qty int64) order.CreditNoteIssued {
	b.t.Helper()
	events := make([]order.Event, len(b.events))
	for i, se := range b.events {
		events[i] = se.Event
	}
	st, err := order.Project(events, gstCafe)
	if err != nil {
		b.t.Fatal(err)
	}
	plan, err := order.PlanCredit(st, order.CreditRequest{Lines: []order.CreditItem{{LineID: lineID, Qty: qty}}})
	if err != nil {
		b.t.Fatal(err)
	}
	cn := order.CreditNoteIssued{
		CreditNo: "CN/26-27/000001", Series: "CN", AgainstInvoiceNo: st.Invoices[len(st.Invoices)-1].InvoiceNo,
		Reason: "cold", AuthorisedBy: 2,
		Lines: plan.Lines, Taxable: plan.Taxable, Tax: plan.Tax, Total: plan.Total, Components: plan.Components,
	}
	b.add(when, cn)
	return cn
}

var menu = map[int64]zreport.Category{1: {ID: 1, Name: "Coffee"}, 7: {ID: 2, Name: "Cakes"}}

// week is a week worked out by hand, Sunday 27 September to Sunday 4
// October 2026, in Kolkata:
//
//   - A, Monday 28th 13:00, counter, opened by 3: 2 coffees at 100.00 (5%),
//     210.00, cash.
//   - B, invoiced Saturday 3rd at 01:30, so Friday 2nd's trade, QR table,
//     opened by 5: a cake at 200.00 (5%), 210.00, card.
//   - C, invoiced Friday 25th, before the range, and one of its two
//     coffees credited on Tuesday 29th: 105.00 back.
//   - D, invoiced Tuesday 6th, after the range.
func week(t *testing.T) [][]shift.StampedEvent {
	a := newBill(t, ist(9, 28, 12, 50), order.ChannelCounter, 3)
	a.line(ist(9, 28, 12, 51), 1, "Coffee", 2, 10000)
	a.add(ist(9, 28, 12, 59), order.PaymentTendered{Method: order.PaymentCash, Amount: inr(21000), Tip: inr(0), TenderedBy: 3})
	a.invoice(ist(9, 28, 13, 0), "INV/26-27/000002")

	b := newBill(t, ist(10, 3, 0, 40), order.ChannelQRTable, 5)
	b.line(ist(10, 3, 0, 41), 7, "Cake", 1, 20000)
	b.add(ist(10, 3, 1, 29), order.PaymentTendered{Method: order.PaymentCard, Amount: inr(21000), Tip: inr(0), TenderedBy: 5})
	b.invoice(ist(10, 3, 1, 30), "INV/26-27/000003")

	c := newBill(t, ist(9, 25, 9, 0), order.ChannelCounter, 3)
	coffee := c.line(ist(9, 25, 9, 1), 1, "Coffee", 2, 10000)
	c.add(ist(9, 25, 9, 5), order.PaymentTendered{Method: order.PaymentCash, Amount: inr(21000), Tip: inr(0), TenderedBy: 3})
	c.invoice(ist(9, 25, 9, 6), "INV/26-27/000001")
	c.credit(ist(9, 29, 11, 0), coffee, 1)

	d := newBill(t, ist(10, 6, 10, 0), order.ChannelCounter, 3)
	d.line(ist(10, 6, 10, 1), 1, "Coffee", 1, 10000)
	d.add(ist(10, 6, 10, 1), order.PaymentTendered{Method: order.PaymentCash, Amount: inr(10500), Tip: inr(0), TenderedBy: 3})
	d.invoice(ist(10, 6, 10, 2), "INV/26-27/000004")

	return [][]shift.StampedEvent{a.events, b.events, c.events, d.events}
}

func build(t *testing.T, every Every, orders [][]shift.StampedEvent) Result {
	t.Helper()
	r, err := Build(Input{
		Range:    Range{From: "2026-09-27", To: "2026-10-04", Every: every, Loc: kolkata, DayStart: shift.DefaultDayStart},
		Currency: "INR", Taxes: gstCafe, Orders: orders, Categories: menu,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTotals(t *testing.T) {
	r := build(t, Day, week(t))
	s := r.Totals.Sales
	if s.Bills != 2 || s.Total != inr(42000) {
		t.Errorf("sales: %d bills, %s; want 2 bills, 420.00", s.Bills, s.Total)
	}
	if r.Totals.CreditNotes.Count != 1 || r.Totals.CreditNotes.Amount != inr(10500) {
		t.Errorf("credit notes: %+v; want one of 105.00", r.Totals.CreditNotes)
	}
	if s.NetOfCredits != inr(31500) {
		t.Errorf("net of credits %s, want 315.00", s.NetOfCredits)
	}
	if len(r.Totals.ByChannel) != 2 || len(r.Totals.ByCategory) != 2 || len(r.Totals.ByItem) != 2 {
		t.Errorf("mix: %d channels, %d categories, %d items; want 2 of each",
			len(r.Totals.ByChannel), len(r.Totals.ByCategory), len(r.Totals.ByItem))
	}
	if len(r.Totals.Payments) != 2 || r.Totals.Balance.Payments != inr(42000) {
		t.Errorf("payments: %+v", r.Totals.Payments)
	}
}

func TestSeriesByDay(t *testing.T) {
	r := build(t, Day, week(t))
	if len(r.Series) != 8 {
		t.Fatalf("%d days, want 8", len(r.Series))
	}
	want := map[string]struct{ bills, total, net int64 }{
		"2026-09-28": {1, 21000, 21000},
		"2026-09-29": {0, 0, -10500},
		"2026-10-02": {1, 21000, 21000}, // invoiced at 01:30 on the 3rd
	}
	for _, p := range r.Series {
		if p.Start != p.End {
			t.Errorf("a day runs %s to %s", p.Start, p.End)
		}
		w := want[p.Start]
		if p.Sales.Bills != w.bills || p.Sales.Total.Minor != w.total || p.Sales.NetOfCredits.Minor != w.net {
			t.Errorf("%s: %d bills, %s, net %s; want %+v", p.Start, p.Sales.Bills, p.Sales.Total, p.Sales.NetOfCredits, w)
		}
		if p.Sales.Total.Currency != "INR" {
			t.Errorf("%s: an empty day still carries the currency, got %+v", p.Start, p.Sales.Total)
		}
	}
}

func TestSeriesByWeekAndMonth(t *testing.T) {
	tests := []struct {
		every Every
		want  []Point
	}{
		{Week, []Point{
			{Period: Period{"2026-09-27", "2026-09-27"}},
			{Period: Period{"2026-09-28", "2026-10-04"}, Sales: zreport.Sales{Bills: 2}},
		}},
		{Month, []Point{
			{Period: Period{"2026-09-27", "2026-09-30"}, Sales: zreport.Sales{Bills: 1}},
			{Period: Period{"2026-10-01", "2026-10-04"}, Sales: zreport.Sales{Bills: 1}},
		}},
	}
	for _, tt := range tests {
		t.Run(string(tt.every), func(t *testing.T) {
			r := build(t, tt.every, week(t))
			if len(r.Series) != len(tt.want) {
				t.Fatalf("%d periods, want %d: %+v", len(r.Series), len(tt.want), r.Series)
			}
			for i, p := range r.Series {
				if p.Period != tt.want[i].Period || p.Sales.Bills != tt.want[i].Sales.Bills {
					t.Errorf("period %d: %+v with %d bills, want %+v", i, p.Period, p.Sales.Bills, tt.want[i])
				}
			}
		})
	}
}

func TestHours(t *testing.T) {
	r := build(t, Day, week(t))
	want := []Hour{
		{Weekday: 1, Hour: 13, Bills: 1, Total: inr(21000)}, // Monday lunch
		{Weekday: 5, Hour: 1, Bills: 1, Total: inr(21000)},  // Friday night, after midnight
	}
	if len(r.Hours) != len(want) {
		t.Fatalf("hours %+v, want %+v", r.Hours, want)
	}
	for i := range want {
		if r.Hours[i] != want[i] {
			t.Errorf("cell %d: %+v, want %+v", i, r.Hours[i], want[i])
		}
	}
}

// A range whose every event is in one shift adds up to that shift's
// Z-report: the two never tell an owner different figures.
func TestAgreesWithTheZReport(t *testing.T) {
	orders := week(t)[:2]
	r := build(t, Day, orders)
	z, err := zreport.Build(zreport.Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Orders: orders, Categories: menu})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(r.Totals)
	b, _ := json.Marshal(z)
	if string(a) != string(b) {
		t.Errorf("range totals\n%s\nZ-report\n%s", a, b)
	}
}

func TestEmptyRange(t *testing.T) {
	r := build(t, Day, nil)
	if r.Totals.Sales.Bills != 0 || r.Totals.Sales.Total != inr(0) || len(r.Series) != 8 || len(r.Hours) != 0 {
		t.Errorf("an empty week: %+v", r)
	}
}

func TestBadRanges(t *testing.T) {
	ok := Range{From: "2026-09-27", To: "2026-10-04", Every: Day, Loc: kolkata, DayStart: shift.DefaultDayStart}
	tests := []struct {
		name string
		edit func(r Range) Range
		ok   bool
	}{
		{"a good week", func(r Range) Range { return r }, true},
		{"one day", func(r Range) Range { r.To = r.From; return r }, true},
		{"a whole leap-free year of 366 days", func(r Range) Range { r.From, r.To = "2026-01-01", "2027-01-01"; return r }, true},
		{"367 days", func(r Range) Range { r.From, r.To = "2026-01-01", "2027-01-02"; return r }, false},
		{"backwards", func(r Range) Range { r.From, r.To = r.To, r.From; return r }, false},
		{"not a day", func(r Range) Range { r.From = "27/09/2026"; return r }, false},
		{"no timezone", func(r Range) Range { r.Loc = nil; return r }, false},
		{"a fortnight", func(r Range) Range { r.Every = "fortnight"; return r }, false},
		{"a day start past the day", func(r Range) Range { r.DayStart = 25 * time.Hour; return r }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.edit(ok).Periods()
			if tt.ok && err != nil {
				t.Errorf("refused: %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrBadRange) {
				t.Errorf("err %v, want ErrBadRange", err)
			}
		})
	}
}

// Each period says how its sales were served, and the ways come to the
// period's own total: the trend chart stacks them (D15).
func TestSeriesByService(t *testing.T) {
	r := build(t, Day, week(t))
	sold := 0
	for _, p := range r.Series {
		sum, bills := int64(0), int64(0)
		for _, g := range p.ByService {
			sum += g.Total.Minor
			bills += g.Qty
		}
		if sum != p.Sales.Total.Minor || bills != p.Sales.Bills {
			t.Errorf("%s: by service %d over %d bills; the period is %s over %d", p.Start, sum, bills, p.Sales.Total, p.Sales.Bills)
		}
		if p.ByService == nil {
			t.Errorf("%s: by_service is null; a quiet period is an empty list", p.Start)
		}
		if p.Sales.Bills > 0 {
			sold++
		}
	}
	if sold == 0 {
		t.Fatal("the week sold nothing: the test proves nothing")
	}
	total := int64(0)
	for _, g := range r.Totals.ByService {
		total += g.Total.Minor
	}
	if total != r.Totals.Sales.Total.Minor {
		t.Errorf("the range by service comes to %d, its sales to %s", total, r.Totals.Sales.Total)
	}
}
