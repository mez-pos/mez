package report

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
)

// The week of report_test.go, worked out by hand: two bills of 200.00 plus
// 5%, a coffee credited (105.00) from a bill before the range.
func TestTables(t *testing.T) {
	r := build(t, Month, week(t))
	tests := []struct {
		sheet  Sheet
		header []string
		rows   [][]string // each must be in the table
		n      int        // how many rows; -1 to leave unchecked
	}{
		{SheetSummary, []string{"Figure", "Count", "Amount (INR)"}, [][]string{
			{"Trading days", "2026-09-27 to 2026-10-04", ""},
			{"Bills", "2", ""},
			{"Gross sales", "", "400.00"},
			{"Tax", "", "20.00"},
			{"Total", "", "420.00"},
			{"Credit notes", "1", "105.00"},
			{"Total net of credit notes", "", "315.00"},
			{"Payments", "", "420.00"},
		}, -1},
		{SheetOverTime, []string{"From", "To", "Bills", "Gross (INR)", "Discounts (INR)", "Net (INR)", "Taxable (INR)", "Tax (INR)", "Total (INR)", "Net of credit notes (INR)"}, [][]string{
			{"2026-09-27", "2026-09-30", "1", "200.00", "0.00", "200.00", "200.00", "10.00", "210.00", "105.00"},
			{"2026-10-01", "2026-10-04", "1", "200.00", "0.00", "200.00", "200.00", "10.00", "210.00", "210.00"},
		}, 2},
		{SheetItems, []string{"Item", "Qty", "Gross (INR)", "Discounts (INR)", "Net (INR)", "Tax (INR)", "Total (INR)"}, [][]string{
			{"Coffee", "2", "200.00", "0.00", "200.00", "10.00", "210.00"},
			{"Cake", "1", "200.00", "0.00", "200.00", "10.00", "210.00"},
		}, 2},
		{SheetCategories, []string{"Category", "Qty", "Gross (INR)", "Discounts (INR)", "Net (INR)", "Tax (INR)", "Total (INR)"}, [][]string{
			{"Cakes", "1", "200.00", "0.00", "200.00", "10.00", "210.00"},
		}, 2},
		{SheetChannels, []string{"Channel", "Bills", "Gross (INR)", "Discounts (INR)", "Net (INR)", "Tax (INR)", "Total (INR)"}, nil, 2},
		{SheetStaff, []string{"Opened by", "Bills", "Gross (INR)", "Discounts (INR)", "Net (INR)", "Tax (INR)", "Total (INR)"}, nil, 2},
		{SheetPayments, []string{"Method", "Tenders", "Taken (INR)", "Voided (INR)", "Refunded (INR)", "Net (INR)"}, [][]string{
			{"cash", "1", "210.00", "0.00", "0.00", "210.00"},
			{"card", "1", "210.00", "0.00", "0.00", "210.00"},
		}, 2},
		{SheetTaxes, []string{"Tax", "Rate", "Taxable (INR)", "Amount (INR)"}, nil, -1},
		// B was invoiced at 01:30 on the Saturday: Friday night's trade.
		{SheetHours, []string{"Weekday", "Hour", "Bills", "Total (INR)"}, [][]string{
			{"Monday", "13:00", "1", "210.00"},
			{"Friday", "01:00", "1", "210.00"},
		}, 2},
	}
	if len(tests) != len(Sheets()) {
		t.Errorf("%d sheets tested of %d", len(tests), len(Sheets()))
	}
	for _, tt := range tests {
		t.Run(string(tt.sheet), func(t *testing.T) {
			got, err := r.Table(tt.sheet)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Header, tt.header) {
				t.Errorf("header %q, want %q", got.Header, tt.header)
			}
			if tt.n >= 0 && len(got.Rows) != tt.n {
				t.Errorf("%d rows, want %d: %q", len(got.Rows), tt.n, got.Rows)
			}
			for _, row := range got.Rows {
				if len(row) != len(got.Header) {
					t.Errorf("row %q has %d cells under %d headings", row, len(row), len(got.Header))
				}
			}
			for _, want := range tt.rows {
				if !slices.ContainsFunc(got.Rows, func(row []string) bool { return slices.Equal(row, want) }) {
					t.Errorf("no row %q in %q", want, got.Rows)
				}
			}
			if _, err := got.CSV(); err != nil {
				t.Errorf("as CSV: %v", err)
			}
		})
	}
}

func TestSundayIsNamed(t *testing.T) {
	r := Result{Hours: []Hour{{Weekday: 7, Hour: 9, Bills: 1, Total: inr(100)}}}
	got, err := r.Table(SheetHours)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows[0][0] != "Sunday" || got.Rows[0][1] != "09:00" {
		t.Errorf("row %q, want Sunday at 09:00", got.Rows[0])
	}
}

func TestAQuietRangeIsAHeaderAlone(t *testing.T) {
	r, err := Build(Input{
		Range:    Range{From: "2026-08-01", To: "2026-08-02", Every: Day, Loc: kolkata, DayStart: shift.DefaultDayStart},
		Currency: "INR", Taxes: gstCafe, Categories: menu,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Table(SheetItems)
	if err != nil {
		t.Fatal(err)
	}
	body, err := got.CSV()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "Item,Qty,Gross (INR),Discounts (INR),Net (INR),Tax (INR),Total (INR)\r\n" {
		t.Errorf("csv %q", body)
	}
}

func TestAnItemNamedLikeAFormulaStaysText(t *testing.T) {
	b := newBill(t, ist(9, 28, 12, 50), order.ChannelCounter, 3)
	b.line(ist(9, 28, 12, 51), 1, "=1+1", 1, 10000)
	b.add(ist(9, 28, 12, 59), order.PaymentTendered{Method: order.PaymentCash, Amount: inr(10500), Tip: inr(0), TenderedBy: 3})
	b.invoice(ist(9, 28, 13, 0), "INV/26-27/000009")
	got, err := build(t, Day, [][]shift.StampedEvent{b.events}).Table(SheetItems)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || !strings.HasPrefix(got.Rows[0][0], "'=") {
		t.Errorf("rows %q, want the name behind an apostrophe", got.Rows)
	}
}

func TestNoSuchSheet(t *testing.T) {
	for _, s := range []Sheet{"", "profit", "Items"} {
		if _, err := (Result{}).Table(s); !errors.Is(err, ErrNoSheet) {
			t.Errorf("%q: err %v, want ErrNoSheet", s, err)
		}
		if err := s.Check(); !errors.Is(err, ErrNoSheet) {
			t.Errorf("%q: Check %v, want ErrNoSheet", s, err)
		}
	}
}
