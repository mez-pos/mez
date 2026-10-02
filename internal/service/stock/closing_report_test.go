package stock

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

const ml = stock.PerUnit

// shiftOn opens a shift on a trading day; closeShift closes it by hand, so
// the next can open.
func (f fixture) shiftOn(t *testing.T, publicID, day string) int64 {
	t.Helper()
	res, err := f.d.Writer().ExecContext(context.Background(),
		`INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at, closed_by, closed_at)
		 VALUES (?, 1, 'open', ?, 1, ? || ' 06:00:00+00:00', 0, NULL)`, publicID, day, day)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f fixture) closeShift(t *testing.T, id int64) {
	t.Helper()
	if _, err := f.d.Writer().ExecContext(context.Background(),
		`UPDATE shifts SET status = 'closed', closed_by = 1, closed_at = trading_day || ' 18:00:00+00:00' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
}

// twoCloses is two nights, worked out by hand. Milk is 60.00 a litre the
// first night and 64.00 the second; beans have no cost.
//
//	29 Sept  Beans  2000 g expected, 1900 counted: 100 g short, no cost
//	         Milk   5000 ml expected, 4500 counted: 500 ml short, 30.00
//	30 Sept  Beans  1900 g expected, 1900 counted: none
//	         Milk   4500 ml expected, 4600 counted: 100 ml over, 6.40
func twoCloses(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	milk, err := f.s.CreateItem(ctx, 1, ItemInput{Name: "Milk", Unit: stock.Millilitre, CountAtClose: true, Cost: inr(6000)})
	if err != nil {
		t.Fatal(err)
	}
	beans := f.flagged(t, 1, "Beans", stock.Gram)
	f.record(t, milk, MovementInput{Kind: stock.Received, Qty: 5000 * ml})
	f.record(t, beans, MovementInput{Kind: stock.Received, Qty: 2 * kg})

	first := f.shiftOn(t, "shf_NIGHT001", "2026-09-29")
	if _, err := f.s.RecordClosingCount(ctx, 1, f.asha, first, []ClosingCountInput{{milk, 4500 * ml}, {beans, 1900 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}
	f.closeShift(t, first)
	if err := f.s.UpdateItem(ctx, 1, milk, ItemInput{Name: "Milk", Unit: stock.Millilitre, CountAtClose: true, Cost: inr(6400)}); err != nil {
		t.Fatal(err)
	}
	second := f.shiftOn(t, "shf_NIGHT002", "2026-09-30")
	if _, err := f.s.RecordClosingCount(ctx, 1, f.asha, second, []ClosingCountInput{{milk, 4600 * ml}, {beans, 1900 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}
	f.closeShift(t, second)
	return f
}

func TestTheClosingReport(t *testing.T) {
	f := twoCloses(t)
	got, err := f.s.ClosingReport(context.Background(), 1, "2026-09-29", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if got.From != "2026-09-29" || got.To != "2026-09-30" || got.Currency != "INR" {
		t.Errorf("range %s to %s in %s", got.From, got.To, got.Currency)
	}
	if got.Value != inr(-2360) || got.Uncosted != 2 {
		t.Errorf("total %+v with %d uncosted; want 23.60 lost and two counts with no cost", got.Value, got.Uncosted)
	}
	if len(got.Shifts) != 2 {
		t.Fatalf("%d shifts, want 2", len(got.Shifts))
	}
	amt := func(minor int64) *money.Amount { a := inr(minor); return &a }
	same := func(a, b *money.Amount) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }
	wantShifts := []struct {
		id, day  string
		value    int64
		uncosted int64
		lines    []ClosingLine
	}{
		{"shf_NIGHT001", "2026-09-29", -3000, 1, []ClosingLine{
			{Name: "Beans", Unit: stock.Gram, Expected: 2 * kg, Counted: 1900 * stock.PerUnit, Variance: -100 * stock.PerUnit},
			{Name: "Milk", Unit: stock.Millilitre, Expected: 5000 * ml, Counted: 4500 * ml, Variance: -500 * ml, Cost: amt(6000), Value: amt(-3000)},
		}},
		// The second night is valued at 64.00, the cost it was counted at.
		{"shf_NIGHT002", "2026-09-30", 640, 1, []ClosingLine{
			{Name: "Beans", Unit: stock.Gram, Expected: 1900 * stock.PerUnit, Counted: 1900 * stock.PerUnit},
			{Name: "Milk", Unit: stock.Millilitre, Expected: 4500 * ml, Counted: 4600 * ml, Variance: 100 * ml, Cost: amt(6400), Value: amt(640)},
		}},
	}
	for i, want := range wantShifts {
		sh := got.Shifts[i]
		if sh.ShiftID != want.id || sh.TradingDay != want.day || sh.Value != inr(want.value) || sh.Uncosted != want.uncosted || len(sh.Lines) != len(want.lines) {
			t.Errorf("shift %d = %+v, want %+v", i, sh, want)
			continue
		}
		for j, wl := range want.lines {
			l := sh.Lines[j]
			if l.Name != wl.Name || l.Unit != wl.Unit || l.Expected != wl.Expected || l.Counted != wl.Counted || l.Variance != wl.Variance ||
				!same(l.Cost, wl.Cost) || !same(l.Value, wl.Value) || l.StockItemID == 0 {
				t.Errorf("%s, %s = %+v, want %+v", want.day, wl.Name, l, wl)
			}
		}
	}

	// The largest loss first.
	if len(got.Items) != 2 {
		t.Fatalf("items %+v", got.Items)
	}
	if m := got.Items[0]; m.Name != "Milk" || m.Counts != 2 || m.Variance != -400*ml || m.Value != inr(-2360) || m.Uncosted != 0 {
		t.Errorf("milk over the range = %+v; want 400 ml short over two counts, 23.60", m)
	}
	if b := got.Items[1]; b.Name != "Beans" || b.Counts != 2 || b.Variance != -100*stock.PerUnit || b.Value != inr(0) || b.Uncosted != 2 {
		t.Errorf("beans over the range = %+v; want 100 g short, no cost on either count", b)
	}
}

func TestTheClosingReportsRange(t *testing.T) {
	f := twoCloses(t)
	ctx := context.Background()
	tests := []struct {
		name     string
		venue    int64
		from, to string
		shifts   int
		value    int64
	}{
		{"the second night alone", 1, "2026-09-30", "2026-09-30", 1, 640},
		{"the first night alone", 1, "2026-09-29", "2026-09-29", 1, -3000},
		{"a week around them", 1, "2026-09-25", "2026-10-02", 2, -2360},
		{"before either", 1, "2026-09-01", "2026-09-28", 0, 0},
		{"another venue", 2, "2026-09-29", "2026-09-30", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.s.ClosingReport(ctx, tt.venue, tt.from, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Shifts) != tt.shifts || got.Value != inr(tt.value) {
				t.Errorf("%d shifts worth %+v, want %d worth %d", len(got.Shifts), got.Value, tt.shifts, tt.value)
			}
			if got.Shifts == nil || got.Items == nil {
				t.Error("a quiet range still has its lists")
			}
		})
	}
	for _, r := range [][2]string{{"yesterday", "2026-09-30"}, {"2026-09-30", ""}, {"2026-09-30", "2026-09-29"}, {"2025-01-01", "2026-09-30"}} {
		if _, err := f.s.ClosingReport(ctx, 1, r[0], r[1]); !errors.Is(err, ErrBadRange) {
			t.Errorf("%q to %q: err %v, want ErrBadRange", r[0], r[1], err)
		}
		if _, err := f.s.ClosingTable(ctx, 1, r[0], r[1]); !errors.Is(err, ErrBadRange) {
			t.Errorf("%q to %q as a table: err %v, want ErrBadRange", r[0], r[1], err)
		}
	}
}

func TestTheClosingReportAsATable(t *testing.T) {
	f := twoCloses(t)
	got, err := f.s.ClosingTable(context.Background(), 1, "2026-09-29", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	header := []string{"Trading day", "Shift", "Item", "Unit", "Expected", "Counted", "Variance", "Cost (INR)", "Cost of", "Variance at cost (INR)"}
	if !slices.Equal(got.Header, header) {
		t.Errorf("header %q", got.Header)
	}
	want := [][]string{
		// No cost known: empty cells, not 0.00.
		{"2026-09-29", "shf_NIGHT001", "Beans", "g", "2000", "1900", "-100", "", "", ""},
		{"2026-09-29", "shf_NIGHT001", "Milk", "ml", "5000", "4500", "-500", "60.00", "l", "-30.00"},
		{"2026-09-30", "shf_NIGHT002", "Beans", "g", "1900", "1900", "0", "", "", ""},
		{"2026-09-30", "shf_NIGHT002", "Milk", "ml", "4500", "4600", "100", "64.00", "l", "6.40"},
	}
	if !slices.EqualFunc(got.Rows, want, func(a, b []string) bool { return slices.Equal(a, b) }) {
		t.Errorf("rows %q, want %q", got.Rows, want)
	}
	if _, err := got.CSV(); err != nil {
		t.Errorf("as CSV: %v", err)
	}
}

func TestQtyText(t *testing.T) {
	tests := []struct {
		q    stock.Qty
		want string
	}{
		{0, "0"}, {1000, "1"}, {18500, "18.5"}, {250, "0.25"}, {1, "0.001"}, {-500000, "-500"}, {-250, "-0.25"}, {1001, "1.001"}, {1010, "1.01"},
	}
	for _, tt := range tests {
		if got := qtyText(tt.q); got != tt.want {
			t.Errorf("qtyText(%d) = %q, want %q", tt.q, got, tt.want)
		}
	}
}

// closingReport on its own: an item named like a formula, a count by the
// piece, and a value that cannot be worked out.
func TestClosingReportRows(t *testing.T) {
	rows := []gen.ClosingCountsInRangeRow{
		{ShiftID: 7, ShiftPublicID: "shf_A", TradingDay: "2026-10-01", StockItemID: 3, Name: "=Limes", Unit: "each", Delta: -1500, UnitCost: 750},
	}
	rows[0].Counted.Int64, rows[0].Counted.Valid = 10500, true
	got, err := closingReport(rows, "INR")
	if err != nil {
		t.Fatal(err)
	}
	// One and a half limes short at 7.50 each: 11.25.
	if l := got.Shifts[0].Lines[0]; l.Expected != 12000 || l.Value == nil || *l.Value != inr(-1125) {
		t.Errorf("line %+v", l)
	}
	if row := got.Table().Rows[0]; row[2] != "'=Limes" || row[4] != "12" || row[6] != "-1.5" || row[8] != "each" || row[9] != "-11.25" {
		t.Errorf("row %q", row)
	}
	rows[0].Unit = "oz"
	if _, err := closingReport(rows, "INR"); !errors.Is(err, stock.ErrValue) {
		t.Errorf("a unit we do not have: err %v, want ErrValue", err)
	}
}
