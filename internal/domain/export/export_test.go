package export

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/shift"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/tax"
)

// 13:30 in Kolkata: a document is dated in the venue's zone.
var at = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)

var kolkata = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return loc
}()

// bill builds one order's stream, every event in shift 1.
type bill struct {
	t      *testing.T
	taxes  order.Taxes
	cur    string
	events []shift.StampedEvent
}

func newBill(t *testing.T, taxes order.Taxes, cur string, channel order.Channel) *bill {
	b := &bill{t: t, taxes: taxes, cur: cur}
	b.add(order.OrderOpened{Channel: channel, Currency: cur, Covers: 1, OpenedBy: 3})
	return b
}

func (b *bill) m(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: b.cur} }

func (b *bill) add(p order.Payload) int64 {
	seq := int64(len(b.events) + 1)
	b.events = append(b.events, shift.StampedEvent{Event: order.NewEvent(seq, at, 3, 1, p), ShiftID: 1})
	return seq
}

func (b *bill) state() order.State {
	b.t.Helper()
	events := make([]order.Event, len(b.events))
	for i, se := range b.events {
		events[i] = se.Event
	}
	st, err := order.Project(events, b.taxes)
	if err != nil {
		b.t.Fatal(err)
	}
	return st
}

func (b *bill) line(name, hsn string, qty, price int64, rate tax.Rate, exempt bool) int64 {
	return b.add(order.LineAdded{ItemID: 1, Name: name, Qty: qty, UnitPrice: b.m(price), Station: "bar", TaxRate: rate, HSN: hsn, TaxExempt: exempt})
}

// settle pays the whole bill by method and invoices it.
func (b *bill) settle(method order.PaymentMethod, no string) int64 {
	b.t.Helper()
	pay := b.add(order.PaymentTendered{Method: method, Amount: b.state().Totals.Total, Tip: b.m(0), TenderedBy: 3})
	b.add(order.InvoiceIssued{InvoiceNo: no, Series: "INV"})
	return pay
}

func (b *bill) credit(lineID, qty, payment int64, no string) {
	b.t.Helper()
	st := b.state()
	plan, err := order.PlanCredit(st, order.CreditRequest{Lines: []order.CreditItem{{LineID: lineID, Qty: qty}}})
	if err != nil {
		b.t.Fatal(err)
	}
	b.add(order.CreditNoteIssued{
		CreditNo: no, Series: "CN", AgainstInvoiceNo: st.Invoices[len(st.Invoices)-1].InvoiceNo, Reason: "cold", AuthorisedBy: 2,
		Lines: plan.Lines, Taxable: plan.Taxable, Tax: plan.Tax, Total: plan.Total, Components: plan.Components,
	})
	b.add(order.RefundIssued{Amount: plan.Total, AgainstPaymentID: payment, Reason: "cold"})
}

var gstCafe = order.Taxes{Profile: tax.GST{}, Venue: tax.VenueContext{
	Currency: "INR", Pricing: tax.Exclusive, GSTIN: "29ABCDE1234F1Z5", StateCode: "29",
}}

// gstDay: invoice 1, 2 filter coffee at 60 (5%, SAC 996331) and a cake at
// 150 (18%, HSN 1905) with 10% off, 285.30 in cash; invoice 2, 2 lattes at
// 200 (5%), 420.00 by card, one latte credited (210.00) back to the card.
func gstDay(t *testing.T) Input {
	a := newBill(t, gstCafe, "INR", order.ChannelCounter)
	a.line("Filter coffee", "996331", 2, 6000, 5000, false)
	cake := a.line("Carrot cake", "1905", 1, 15000, 18000, false)
	a.add(order.DiscountApplied{Scope: order.ScopeLine, LineID: cake, Kind: order.DiscountPercent, BasisPoints: 1000, Reason: "regular"})
	a.settle(order.PaymentCash, "INV/26-27/000001")

	b := newBill(t, gstCafe, "INR", order.ChannelQRTable)
	latte := b.line("Latte", "996331", 2, 20000, 5000, false)
	card := b.settle(order.PaymentCard, "INV/26-27/000002")
	b.credit(latte, 1, card, "CN/26-27/000001")
	return Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Location: kolkata, Orders: [][]shift.StampedEvent{a.events, b.events}}
}

func TestAGSTRegisterIsPerInvoiceHSNAndRate(t *testing.T) {
	f, err := Build(gstDay(t))
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := []string{"Invoice No.", "Invoice date", "Place of supply", "HSN/SAC", "Rate", "Taxable value", "CGST", "SGST/UTGST", "IGST", "Invoice value"}
	if !slices.Equal(f.Register.Header, wantHeader) {
		t.Errorf("header = %v", f.Register.Header)
	}
	want := [][]string{
		{"INV/26-27/000001", "2026-09-28", "29", "996331", "5%", "120.00", "3.00", "3.00", "0.00", "285.30"},
		{"INV/26-27/000001", "2026-09-28", "29", "1905", "18%", "135.00", "12.15", "12.15", "0.00", "285.30"},
		{"INV/26-27/000002", "2026-09-28", "29", "996331", "5%", "400.00", "10.00", "10.00", "0.00", "420.00"},
		{"CN/26-27/000001", "2026-09-28", "29", "996331", "5%", "-200.00", "-5.00", "-5.00", "0.00", "-210.00"},
	}
	equalRows(t, "register", f.Register.Rows, want)
}

func TestTallyVouchersBalance(t *testing.T) {
	f, err := Build(gstDay(t))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "Cash", "285.30", ""},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "Sales @ 5%", "", "120.00"},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "Sales @ 18%", "", "135.00"},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "CGST @ 2.5%", "", "3.00"},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "SGST @ 2.5%", "", "3.00"},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "CGST @ 9%", "", "12.15"},
		{"28-09-2026", "Sales", "INV/26-27/000001", "", "SGST @ 9%", "", "12.15"},
		{"28-09-2026", "Sales", "INV/26-27/000002", "", "Card", "420.00", ""},
		{"28-09-2026", "Sales", "INV/26-27/000002", "", "Sales @ 5%", "", "400.00"},
		{"28-09-2026", "Sales", "INV/26-27/000002", "", "CGST @ 2.5%", "", "10.00"},
		{"28-09-2026", "Sales", "INV/26-27/000002", "", "SGST @ 2.5%", "", "10.00"},
		{"28-09-2026", "Credit Note", "CN/26-27/000001", "INV/26-27/000002", "Card", "", "210.00"},
		{"28-09-2026", "Credit Note", "CN/26-27/000001", "INV/26-27/000002", "Sales @ 5%", "200.00", ""},
		{"28-09-2026", "Credit Note", "CN/26-27/000001", "INV/26-27/000002", "CGST @ 2.5%", "5.00", ""},
		{"28-09-2026", "Credit Note", "CN/26-27/000001", "INV/26-27/000002", "SGST @ 2.5%", "5.00", ""},
	}
	equalRows(t, "tally", f.Tally.Rows, want)
}

func TestTheDaysSales(t *testing.T) {
	f, err := Build(gstDay(t))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"2026-09-28", "13:30", "Invoice", "INV/26-27/000001", "", "counter", "3", "270.00", "15.00", "255.00", "30.30", "285.30", "285.30", "", "", ""},
		{"2026-09-28", "13:30", "Invoice", "INV/26-27/000002", "", "qr_table", "3", "400.00", "0.00", "400.00", "20.00", "420.00", "", "420.00", "", ""},
		{"2026-09-28", "13:30", "Credit note", "CN/26-27/000001", "INV/26-27/000002", "qr_table", "3", "", "", "-200.00", "-10.00", "-210.00", "", "-210.00", "", ""},
	}
	equalRows(t, "sales", f.Sales.Rows, want)
	if len(f.Sales.Header) != len(want[0]) {
		t.Errorf("header has %d columns, rows %d", len(f.Sales.Header), len(want[0]))
	}
}

// A VAT venue gets VAT columns, from the same code.
func TestAVATRegister(t *testing.T) {
	vat := order.Taxes{Profile: tax.VAT{}, Venue: tax.VenueContext{Currency: "GBP", Pricing: tax.Inclusive, VATNumber: "GB123456789"}}
	a := newBill(t, vat, "GBP", order.ChannelCounter)
	a.line("Flat white", "", 2, 500, 20000, false)
	a.line("Water", "", 1, 150, 0, false)
	a.settle(order.PaymentCard, "R-000001")
	f, err := Build(Input{ShiftID: 1, Currency: "GBP", Taxes: vat, Location: time.UTC, Orders: [][]shift.StampedEvent{a.events}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.Register.Header, []string{"Receipt No.", "Date", "VAT rate", "Net", "VAT", "Gross"}) {
		t.Errorf("header = %v", f.Register.Header)
	}
	// 10.00 at 20% inclusive: 8.33 net, 1.67 VAT. Water at 0%: 1.50.
	want := [][]string{
		{"R-000001", "2026-09-28", "20%", "8.33", "1.67", "10.00"},
		{"R-000001", "2026-09-28", "0%", "1.50", "0.00", "1.50"},
	}
	equalRows(t, "register", f.Register.Rows, want)
}

// A US venue's register is by day and authority: two invoices, one row
// per authority, and exempt sales apart.
func TestAUSRegisterIsPerDayAndAuthority(t *testing.T) {
	us := order.Taxes{Profile: tax.USSalesTax{}, Venue: tax.VenueContext{
		Currency: "USD", Pricing: tax.Exclusive,
		Jurisdiction: []tax.Component{{Name: "New York State", Rate: 4000}, {Name: "New York City", Rate: 4500}},
	}}
	a := newBill(t, us, "USD", order.ChannelCounter)
	a.line("Bagel", "", 1, 1000, 0, false)
	a.line("Bottled water", "", 1, 200, 0, true)
	a.settle(order.PaymentCard, "S-1")
	b := newBill(t, us, "USD", order.ChannelCounter)
	b.line("Bagel", "", 2, 1000, 0, false)
	b.settle(order.PaymentCash, "S-2")
	f, err := Build(Input{ShiftID: 1, Currency: "USD", Taxes: us, Location: time.UTC, Orders: [][]shift.StampedEvent{a.events, b.events}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.Register.Header, []string{"Date", "Jurisdiction", "Rate", "Taxable sales", "Exempt sales", "Tax collected"}) {
		t.Errorf("header = %v", f.Register.Header)
	}
	// 30.00 taxed at 4% state (1.20) and 4.5% city (1.35); 2.00 exempt.
	want := [][]string{
		{"2026-09-28", "New York State", "4%", "30.00", "0.00", "1.20"},
		{"2026-09-28", "New York City", "4.5%", "30.00", "0.00", "1.35"},
		{"2026-09-28", "", "0%", "0.00", "2.00", "0.00"},
	}
	equalRows(t, "register", f.Register.Rows, want)
	// The Tally side still balances with two authorities and an exempt line.
	for _, r := range f.Tally.Rows {
		if r[4] == "Sales, exempt" && r[6] != "2.00" {
			t.Errorf("exempt sales = %v", r)
		}
	}
}

// An untaxed venue still exports what it sold.
func TestAnUntaxedVenue(t *testing.T) {
	none := order.Taxes{}
	a := newBill(t, none, "INR", order.ChannelCounter)
	a.line("Tea", "", 2, 2000, 0, false)
	a.settle(order.PaymentUPI, "INV/1")
	f, err := Build(Input{ShiftID: 1, Currency: "INR", Taxes: none, Location: kolkata, Orders: [][]shift.StampedEvent{a.events}})
	if err != nil {
		t.Fatal(err)
	}
	equalRows(t, "register", f.Register.Rows, [][]string{{"INV/1", "2026-09-28", "40.00"}})
	equalRows(t, "tally", f.Tally.Rows, [][]string{
		{"28-09-2026", "Sales", "INV/1", "", "UPI", "40.00", ""},
		{"28-09-2026", "Sales", "INV/1", "", "Sales", "", "40.00"},
	})
}

// Every profile's columns can be filled: a new regime's register is filled
// on purpose, or this fails.
func TestEveryProfilesColumnsAreKnown(t *testing.T) {
	for _, p := range []tax.TaxProfile{tax.GST{}, tax.VAT{}, tax.USSalesTax{}} {
		for _, c := range p.RegisterColumns() {
			if !slices.Contains(dimensionKeys, c.Key) && !slices.Contains(amountKeys, c.Key) {
				t.Errorf("%s: column %q is not filled", p.Name(), c.Key)
			}
		}
	}
}

func TestOnlyTheShiftsDocumentsAreExported(t *testing.T) {
	in := gstDay(t)
	for i := range in.Orders[0] {
		in.Orders[0][i].ShiftID = 0 // the first bill was another shift's
	}
	f, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range f.Register.Rows {
		if r[0] == "INV/26-27/000001" {
			t.Errorf("another shift's invoice is in the register: %v", r)
		}
	}
}

func TestCSV(t *testing.T) {
	tb := Table{Header: []string{"Ledger", "Debit"}, Rows: [][]string{{"Sales, exempt", "2.00"}, {`Say "hi"`, ""}}}
	data, err := tb.CSV()
	if err != nil {
		t.Fatal(err)
	}
	want := "Ledger,Debit\r\n\"Sales, exempt\",2.00\r\n\"Say \"\"hi\"\"\",\r\n"
	if string(data) != want {
		t.Errorf("csv = %q, want %q", data, want)
	}
}

func TestBuildRefusals(t *testing.T) {
	if _, err := Build(Input{Currency: "INR"}); !errors.Is(err, ErrExport) {
		t.Errorf("no timezone: err = %v", err)
	}
	if _, err := Build(Input{Currency: "inr", Location: time.UTC}); !errors.Is(err, ErrExport) {
		t.Errorf("a malformed currency: err = %v", err)
	}
	a := newBill(t, gstCafe, "INR", order.ChannelCounter)
	a.add(order.PaymentVoided{PaymentID: 9, Reason: "x", AuthorisedBy: 2})
	if _, err := Build(Input{ShiftID: 1, Currency: "INR", Taxes: gstCafe, Location: kolkata, Orders: [][]shift.StampedEvent{a.events}}); !errors.Is(err, ErrExport) {
		t.Errorf("a stream the projection refuses: err = %v", err)
	}
}

func equalRows(t *testing.T, name string, got, want [][]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s has %d rows, want %d:\n%s", name, len(got), len(want), dump(got))
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("%s row %d = %v\n want %v", name, i, got[i], want[i])
		}
	}
}

func dump(rows [][]string) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Join(r, " | "))
		b.WriteString("\n")
	}
	return b.String()
}

func TestTextKeepsAFormulaAsText(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"a name", "Masala Dosa", "Masala Dosa"},
		{"nothing", "", ""},
		{"a formula", "=HYPERLINK(\"http://x\")", "'=HYPERLINK(\"http://x\")"},
		{"a plus", "+1", "'+1"},
		{"a minus", "-1", "'-1"},
		{"an at", "@SUM(A1)", "'@SUM(A1)"},
		{"a tab", "	=1", "'	=1"},
		{"a sign inside", "half-price", "half-price"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.in); got != tt.want {
				t.Errorf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
