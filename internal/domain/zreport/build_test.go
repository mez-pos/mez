package zreport

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

var at = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// gstCafe is a Bengaluru cafe pricing GST exclusive.
var gstCafe = order.Taxes{Profile: tax.GST{}, Venue: tax.VenueContext{
	Currency: "INR", Pricing: tax.Exclusive, GSTIN: "29ABCDE1234F1Z5", StateCode: "29",
}}

// bill builds one order's stream, event by event, each stamped with the
// shift it is recorded in.
type bill struct {
	t      *testing.T
	shift  int64
	events []shift.StampedEvent
}

func newBill(t *testing.T, shiftID int64, channel order.Channel, openedBy int64) *bill {
	b := &bill{t: t, shift: shiftID}
	b.as(openedBy, order.OrderOpened{Channel: channel, Currency: "INR", TableID: 4, Covers: 2, OpenedBy: openedBy})
	return b
}

// in moves the next events to another shift.
func (b *bill) in(shiftID int64) *bill { b.shift = shiftID; return b }

func (b *bill) as(actor int64, p order.Payload) int64 {
	seq := int64(len(b.events) + 1)
	b.events = append(b.events, shift.StampedEvent{Event: order.NewEvent(seq, at, actor, 1, p), ShiftID: b.shift})
	return seq
}

func (b *bill) add(p order.Payload) int64 { return b.as(3, p) }

func (b *bill) state() order.State {
	b.t.Helper()
	events := make([]order.Event, len(b.events))
	for i, se := range b.events {
		events[i] = se.Event
	}
	st, err := order.Project(events, gstCafe)
	if err != nil {
		b.t.Fatal(err)
	}
	return st
}

func (b *bill) line(item int64, name string, qty, price int64, rate tax.Rate) int64 {
	return b.add(order.LineAdded{ItemID: item, Name: name, Qty: qty, UnitPrice: inr(price), Station: "bar", TaxRate: rate, HSN: "996331"})
}

func (b *bill) invoice(no string) {
	b.add(order.InvoiceIssued{InvoiceNo: no, Series: "INV", TaxSnapshot: json.RawMessage(`{}`)})
}

// credit issues a credit note for some units of a line, planned as the
// till plans one.
func (b *bill) credit(lineID, qty int64) order.CreditNoteIssued {
	b.t.Helper()
	st := b.state()
	plan, err := order.PlanCredit(st, order.CreditRequest{Lines: []order.CreditItem{{LineID: lineID, Qty: qty}}})
	if err != nil {
		b.t.Fatal(err)
	}
	cn := order.CreditNoteIssued{
		CreditNo: "CN/26-27/000001", Series: "CN", AgainstInvoiceNo: st.Invoices[len(st.Invoices)-1].InvoiceNo,
		Reason: "cold", AuthorisedBy: 2,
		Lines: plan.Lines, Taxable: plan.Taxable, Tax: plan.Tax, Total: plan.Total, Components: plan.Components,
	}
	b.as(2, cn)
	return cn
}

const (
	gst5  = tax.Rate(5000)
	gst18 = tax.Rate(18000)
)

var menu = map[int64]Category{
	1: {ID: 1, Name: "Coffee"}, 2: {ID: 1, Name: "Coffee"}, 3: {ID: 1, Name: "Coffee"}, 4: {ID: 1, Name: "Coffee"},
	7: {ID: 2, Name: "Cakes"},
}

// handDay is a day worked out by hand, bill by bill:
//
// A, counter, opened by 3: 2 filter coffee at 60 (5%), a cake at 150 (18%)
// with 10% off, an espresso rung up by mistake and removed. Gross 270.00,
// discount 15.00. Tax: coffee 120.00 at 5% = 6.00; cake 135.00 at 18% =
// 24.30. Total 255.00 + 30.30 = 285.30, paid in cash with a 20.00 tip.
//
// B, QR table, opened by 5: 2 lattes at 200 (5%), a cappuccino at 190
// comped, 20.00 off the order. Gross 400.00, discount 20.00, taxable
// 380.00, tax 19.00, total 399.00 by card. Later one latte is credited:
// 190.00 + 9.50 tax = 199.50, refunded to the card.
//
// C, counter, opened by 3: an espresso at 120 (5%), 126.00 in cash, the
// cash voided (wrong bill), then paid by UPI.
func handDay(t *testing.T) Input {
	a := newBill(t, 1, order.ChannelCounter, 3)
	a.line(1, "Filter coffee", 2, 6000, gst5)
	cake := a.line(7, "Carrot cake", 1, 15000, gst18)
	mistake := a.line(2, "Espresso", 1, 12000, gst5)
	a.add(order.LineVoided{LineID: mistake, ReasonCode: order.ReasonEnteredInError, AuthorisedBy: 3})
	a.add(order.DiscountApplied{Scope: order.ScopeLine, LineID: cake, Kind: order.DiscountPercent, BasisPoints: 1000, Reason: "regular"})
	a.add(order.PaymentTendered{Method: order.PaymentCash, Amount: inr(28530), Tip: inr(2000), TenderedBy: 3})
	a.invoice("INV/26-27/000001")
	a.as(2, order.InvoiceReprinted{InvoiceNo: "INV/26-27/000001", Reason: "guest asked", AuthorisedBy: 2})

	b := newBill(t, 1, order.ChannelQRTable, 5)
	latte := b.line(3, "Latte", 2, 20000, gst5)
	capp := b.line(4, "Cappuccino", 1, 19000, gst5)
	b.as(2, order.LineComped{LineID: capp, ReasonCode: "birthday", AuthorisedBy: 2})
	off := inr(2000)
	b.add(order.DiscountApplied{Scope: order.ScopeOrder, Kind: order.DiscountAmount, Amount: &off, Reason: "late"})
	card := b.add(order.PaymentTendered{Method: order.PaymentCard, Amount: inr(39900), Tip: inr(0), TenderedBy: 3})
	b.invoice("INV/26-27/000002")
	cn := b.credit(latte, 1)
	b.as(2, order.RefundIssued{Amount: cn.Total, AgainstPaymentID: card, Reason: "cold"})

	c := newBill(t, 1, order.ChannelCounter, 3)
	c.line(2, "Espresso", 1, 12000, gst5)
	cash := c.add(order.PaymentTendered{Method: order.PaymentCash, Amount: inr(12600), Tip: inr(0), TenderedBy: 3})
	c.as(2, order.PaymentVoided{PaymentID: cash, Reason: "wrong bill", AuthorisedBy: 2})
	c.add(order.PaymentTendered{Method: order.PaymentUPI, Amount: inr(12600), Tip: inr(0), TenderedBy: 3})
	c.invoice("INV/26-27/000003")

	return Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Orders: [][]shift.StampedEvent{a.events, b.events, c.events}, Categories: menu}
}

func TestAHandComputedDayBalancesToThePaisa(t *testing.T) {
	r, err := Build(handDay(t))
	if err != nil {
		t.Fatal(err)
	}

	// 285.30 + 399.00 + 126.00 = 810.30 invoiced; 199.50 credited.
	// Gross 270 + 400 + 120 = 790.00; tax 30.30 + 19.00 + 6.00 = 55.30.
	want := Sales{
		Bills: 3, Gross: inr(79000), Discounts: inr(3500), Net: inr(75500),
		Taxable: inr(75500), Tax: inr(5530), Total: inr(81030), NetOfCredits: inr(61080),
	}
	if r.Sales != want {
		t.Errorf("sales = %+v\n want %+v", r.Sales, want)
	}
	if r.Reprints != 1 {
		t.Errorf("reprints = %d, want 1", r.Reprints)
	}
	if r.Invoices != (Invoices{Count: 3, First: "INV/26-27/000001", Last: "INV/26-27/000003"}) {
		t.Errorf("invoices = %+v", r.Invoices)
	}

	tallies := []struct {
		name string
		got  Tally
		want Tally
	}{
		{"discounts", r.Discounts, Tally{2, inr(3500)}},
		{"comps", r.Comps, Tally{1, inr(19000)}},
		{"voids", r.Voids, Tally{0, inr(0)}},
		{"credit notes", r.CreditNotes, Tally{1, inr(19950)}},
		{"refunds", r.Refunds, Tally{1, inr(19950)}},
	}
	for _, tt := range tallies {
		if tt.got != tt.want {
			t.Errorf("%s = %+v, want %+v", tt.name, tt.got, tt.want)
		}
	}
	if len(r.Removed) != 1 || r.Removed[0] != (StaffTally{StaffID: 3, Tally: Tally{1, inr(12000)}}) {
		t.Errorf("removed = %+v, want staff 3 removing 120.00 once", r.Removed)
	}

	// CGST and SGST each: 2.5% on 120 + 380 + 120 - 190 = 430.00 is 10.75
	// (3.00 + 9.50 + 3.00 - 4.75); 9% on 135.00 is 12.15.
	wantTax := []TaxLine{
		{Name: "CGST", Rate: 2500, Taxable: inr(43000), Amount: inr(1075)},
		{Name: "SGST", Rate: 2500, Taxable: inr(43000), Amount: inr(1075)},
		{Name: "CGST", Rate: 9000, Taxable: inr(13500), Amount: inr(1215)},
		{Name: "SGST", Rate: 9000, Taxable: inr(13500), Amount: inr(1215)},
	}
	if !equalSlices(r.Taxes, wantTax) {
		t.Errorf("taxes = %+v\n want %+v", r.Taxes, wantTax)
	}
	taxSum := inr(0)
	for _, tl := range r.Taxes {
		taxSum.Minor += tl.Amount.Minor
	}
	if taxSum.Minor != r.Sales.Tax.Minor-cnTax(t) {
		t.Errorf("the tax lines come to %s; the sales tax less the credited tax is %d", taxSum, r.Sales.Tax.Minor-cnTax(t))
	}

	wantPay := []Payment{
		{Method: order.PaymentCash, Count: 2, Taken: inr(41130), Voided: inr(12600), Refunded: inr(0), Net: inr(28530)},
		{Method: order.PaymentCard, Count: 1, Taken: inr(39900), Voided: inr(0), Refunded: inr(19950), Net: inr(19950)},
		{Method: order.PaymentUPI, Count: 1, Taken: inr(12600), Voided: inr(0), Refunded: inr(0), Net: inr(12600)},
	}
	if !equalSlices(r.Payments, wantPay) {
		t.Errorf("payments = %+v\n want %+v", r.Payments, wantPay)
	}
	if r.Tips != inr(2000) {
		t.Errorf("tips = %s, want 20.00 and apart from the payments", r.Tips)
	}
	if r.Balance != (Balance{Payments: inr(61080), Sales: inr(61080), Difference: inr(0)}) {
		t.Errorf("balance = %+v, want 610.80 paid against 610.80 sold", r.Balance)
	}

	groups := []struct {
		name string
		got  []Group
		want []Group
	}{
		{"categories", r.ByCategory, []Group{
			{Key: "1", Name: "Coffee", Qty: 5, Gross: inr(64000), Discounts: inr(2000), Net: inr(62000), Tax: inr(3100), Total: inr(65100)},
			{Key: "2", Name: "Cakes", Qty: 1, Gross: inr(15000), Discounts: inr(1500), Net: inr(13500), Tax: inr(2430), Total: inr(15930)},
		}},
		{"channels", r.ByChannel, []Group{
			{Key: "counter", Name: "counter", Qty: 2, Gross: inr(39000), Discounts: inr(1500), Net: inr(37500), Tax: inr(3630), Total: inr(41130)},
			{Key: "qr_table", Name: "qr_table", Qty: 1, Gross: inr(40000), Discounts: inr(2000), Net: inr(38000), Tax: inr(1900), Total: inr(39900)},
		}},
		{"staff", r.ByStaff, []Group{
			{Key: "3", Name: "3", Qty: 2, Gross: inr(39000), Discounts: inr(1500), Net: inr(37500), Tax: inr(3630), Total: inr(41130)},
			{Key: "5", Name: "5", Qty: 1, Gross: inr(40000), Discounts: inr(2000), Net: inr(38000), Tax: inr(1900), Total: inr(39900)},
		}},
	}
	for _, g := range groups {
		if !equalSlices(g.got, g.want) {
			t.Errorf("%s = %+v\n want %+v", g.name, g.got, g.want)
		}
	}
	for _, heading := range [][]Group{r.ByCategory, r.ByItem, r.ByChannel, r.ByStaff} {
		sum := int64(0)
		for _, g := range heading {
			sum += g.Total.Minor
		}
		if sum != r.Sales.Total.Minor {
			t.Errorf("a heading comes to %d, the sales to %d", sum, r.Sales.Total.Minor)
		}
	}
	if len(r.ByItem) != 4 || r.ByItem[0].Name != "Latte" || r.ByItem[0].Qty != 2 {
		t.Errorf("items = %+v, want four, the lattes first", r.ByItem)
	}
}

// cnTax is the hand day's credited tax: 4.75 CGST and 4.75 SGST.
func cnTax(t *testing.T) int64 { t.Helper(); return 950 }

// Only what was recorded in the shift counts; a bill invoiced in another
// shift is not a sale here, but its refund and the deposit taken here are.
func TestOnlyTheShiftsEventsCount(t *testing.T) {
	// Invoiced and paid yesterday (shift 1), a latte credited today (2).
	old := newBill(t, 1, order.ChannelCounter, 3)
	latte := old.line(3, "Latte", 2, 20000, gst5)
	card := old.add(order.PaymentTendered{Method: order.PaymentCard, Amount: inr(42000), Tip: inr(0), TenderedBy: 3})
	old.invoice("INV/26-27/000009")
	old.in(2)
	cn := old.credit(latte, 1)
	old.as(2, order.RefundIssued{Amount: cn.Total, AgainstPaymentID: card, Reason: "cold"})

	// A tab opened today with a deposit, not yet invoiced.
	tab := newBill(t, 2, order.ChannelCounter, 5)
	tab.line(1, "Filter coffee", 1, 6000, gst5)
	tab.add(order.PaymentTendered{Method: order.PaymentCash, Amount: inr(5000), Tip: inr(0), TenderedBy: 5})

	r, err := Build(Input{ShiftID: 2, Currency: "INR", Taxes: gstCafe, Orders: [][]shift.StampedEvent{old.events, tab.events}, Categories: menu})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sales.Bills != 0 || r.Sales.Total != inr(0) || r.Invoices.Count != 0 {
		t.Errorf("sales = %+v, want none: nothing was invoiced today", r.Sales)
	}
	if r.CreditNotes != (Tally{1, inr(21000)}) || r.Refunds != (Tally{1, inr(21000)}) {
		t.Errorf("credit notes %+v, refunds %+v; want today's 210.00 each", r.CreditNotes, r.Refunds)
	}
	// Paid today: 50.00 deposit in, 210.00 back to a card. Sold today:
	// nothing, less 210.00 credited. The difference is the deposit.
	if r.Balance != (Balance{Payments: inr(-16000), Sales: inr(-21000), Difference: inr(5000)}) {
		t.Errorf("balance = %+v, want the 50.00 deposit as the difference", r.Balance)
	}
	if len(r.Taxes) != 2 || r.Taxes[0].Amount != inr(-500) {
		t.Errorf("taxes = %+v, want only the credited tax, negative", r.Taxes)
	}
}

func TestAnEmptyShift(t *testing.T) {
	r, err := Build(Input{ShiftID: 1, Currency: "INR"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sales.Total != inr(0) || r.Balance.Difference != inr(0) || r.Payments == nil || r.ByItem == nil || r.Taxes == nil || r.Removed == nil {
		t.Errorf("an empty report = %+v; want zeros and empty lists, not nil", r)
	}
}

func TestAManagersVoidIsNotARemoval(t *testing.T) {
	a := newBill(t, 1, order.ChannelCounter, 3)
	l := a.line(1, "Filter coffee", 3, 6000, gst5)
	a.as(2, order.LineVoided{LineID: l, ReasonCode: "guest_left", AuthorisedBy: 2})
	r, err := Build(Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Orders: [][]shift.StampedEvent{a.events}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Voids != (Tally{1, inr(18000)}) || len(r.Removed) != 0 {
		t.Errorf("voids %+v, removed %+v; want one manager void of 180.00", r.Voids, r.Removed)
	}
}

func TestBuildRefusals(t *testing.T) {
	if _, err := Build(Input{Currency: "inr"}); err == nil {
		t.Error("a malformed currency: want an error")
	}
	// A stream the projection refuses fails the report, not a line of it.
	a := newBill(t, 1, order.ChannelCounter, 3)
	a.add(order.PaymentVoided{PaymentID: 9, Reason: "x", AuthorisedBy: 2})
	if _, err := Build(Input{ShiftID: 1, Currency: "INR", Orders: [][]shift.StampedEvent{a.events}}); err == nil {
		t.Error("a void of no payment: want an error")
	}
	// A bill in dollars at a rupee venue.
	d := &bill{t: t, shift: 1}
	d.as(3, order.OrderOpened{Channel: order.ChannelCounter, Currency: "USD", OpenedBy: 3})
	d.add(order.PaymentTendered{Method: order.PaymentCash, Amount: money.Amount{Minor: 100, Currency: "USD"}, TenderedBy: 3})
	_, err := Build(Input{ShiftID: 1, Currency: "INR", Orders: [][]shift.StampedEvent{d.events}})
	if err == nil {
		t.Fatal("a dollar bill at a rupee venue: want an error")
	}
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		var pe *order.ProjectionError
		if !errors.As(err, &pe) {
			t.Errorf("err = %v, want a currency mismatch", err)
		}
	}
}

func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
