package closeofday

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/zreport"
	svcshift "github.com/mez-pos/mez/internal/service/shift"
)

var (
	opened = time.Date(2026, 10, 21, 3, 30, 0, 0, time.UTC)
	closed = time.Date(2026, 10, 21, 17, 34, 0, 0, time.UTC)
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func newDB(t *testing.T) (*db.DB, int64, int64) {
	t.Helper()
	ctx := context.Background()
	d, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), db.FileName)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close(context.Background()) })
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var venueID, shiftID int64
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_CLOSE0001", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: opened, UpdatedAt: opened,
		})
		if err != nil {
			return err
		}
		venueID = v.ID
		shiftID, err = q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "sh_CLOSE0001", VenueID: v.ID, TradingDay: "2026-10-21", OpenedBy: 1, OpenedAt: opened})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, venueID, shiftID
}

// fullDay is a report with something on every line of the message.
func fullDay() svcshift.ZDocument {
	return svcshift.ZDocument{
		Shift: svcshift.ZShift{PublicID: "sh_CLOSE0001", TradingDay: "2026-10-21", OpenedBy: 2, OpenedAt: opened, ClosedBy: 1, ClosedAt: closed},
		Report: zreport.Report{
			Currency:    "INR",
			Sales:       zreport.Sales{Bills: 86, Gross: inr(2500000), Discounts: inr(69000), Net: inr(2431000), Total: inr(2431000), NetOfCredits: inr(2411000), Covers: 142},
			Invoices:    zreport.Invoices{Count: 86, First: "INV/26-27/001041", Last: "INV/26-27/001126"},
			Discounts:   zreport.Tally{Count: 3, Amount: inr(69000)},
			Comps:       zreport.Tally{Count: 1, Amount: inr(18000)},
			Voids:       zreport.Tally{Count: 2, Amount: inr(52000)},
			CreditNotes: zreport.Tally{Count: 1, Amount: inr(20000)},
			Refunds:     zreport.Tally{Count: 1, Amount: inr(20000)},
			Payments: []zreport.Payment{
				{Method: order.PaymentCash, Count: 40, Taken: inr(900000), Net: inr(880000)},
				{Method: order.PaymentUPI, Count: 36, Taken: inr(1100000), Net: inr(1100000)},
				{Method: order.PaymentCard, Count: 8, Taken: inr(400000), Net: inr(400000)},
				{Method: order.PaymentOther, Count: 2, Taken: inr(151000), Net: inr(151000)},
			},
			Tips:    inr(12500),
			Balance: zreport.Balance{Payments: inr(2531000), Sales: inr(2411000), Difference: inr(120000)},
		},
		Drawer:      svcshift.ZDrawer{Expected: inr(857000), Counted: inr(845000), Variance: inr(-12000)},
		Names:       map[int64]string{1: "Priya", 2: "Asha"},
		GeneratedBy: 1, GeneratedAt: closed,
	}
}

func addReport(t *testing.T, d *db.DB, venueID, shiftID, number int64, doc svcshift.ZDocument, corrects int64) int64 {
	t.Helper()
	ctx := context.Background()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		params := gen.CreateZReportParams{
			VenueID: venueID, Number: number, ShiftID: shiftID, Reason: doc.Reason, Body: string(body),
			GeneratedBy: doc.GeneratedBy, GeneratedAt: doc.GeneratedAt,
		}
		if corrects != 0 {
			params.CorrectsID = sql.NullInt64{Int64: corrects, Valid: true}
		}
		id, err = q.CreateZReport(ctx, params)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSinceReadsTheMessagesFiguresOffTheReport(t *testing.T) {
	d, venueID, shiftID := newDB(t)
	id := addReport(t, d, venueID, shiftID, 212, fullDay(), 0)

	got, err := Since(context.Background(), d.Queries(), venueID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d closes, want 1", len(got))
	}
	c := got[0]
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"id", c.ID, id}, {"number", c.Number, int64(212)}, {"corrects", c.Corrects, int64(0)},
		{"shift", c.ShiftID, "sh_CLOSE0001"}, {"trading day", c.TradingDay, "2026-10-21"},
		{"opened", c.OpenedAt, opened}, {"closed", c.ClosedAt, closed}, {"closed by", c.ClosedBy, "Priya"},
		{"currency", c.Currency, "INR"},
		{"sales", c.SalesTotal, int64(2431000)}, {"bills", c.Bills, int64(86)}, {"covers", c.Covers, int64(142)},
		{"difference", c.Difference, int64(120000)},
		{"cash", c.Cash, int64(880000)}, {"upi", c.UPI, int64(1100000)}, {"card", c.Card, int64(400000)}, {"other", c.Other, int64(151000)},
		{"counted", c.Counted, int64(845000)}, {"variance", c.Variance, int64(-12000)},
		{"voids", c.Voids, Tally{2, 52000}}, {"comps", c.Comps, Tally{1, 18000}}, {"discounts", c.Discounts, Tally{3, 69000}},
		{"credit notes", c.CreditNotes, Tally{1, 20000}}, {"refunds", c.Refunds, Tally{1, 20000}},
		{"tips", c.Tips, int64(12500)},
		{"invoices", c.Invoices, int64(86)}, {"first", c.FirstInvoice, "INV/26-27/001041"}, {"last", c.LastInvoice, "INV/26-27/001126"},
		{"generated", c.GeneratedAt, closed},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestSinceIsACursorOldestFirst(t *testing.T) {
	d, venueID, shiftID := newDB(t)
	first := addReport(t, d, venueID, shiftID, 1, fullDay(), 0)
	corrected := fullDay()
	corrected.Corrects, corrected.Reason = 1, "a tender voided late"
	second := addReport(t, d, venueID, shiftID, 2, corrected, first)
	// A shift has one original report: the next day's is another shift's.
	ctx := context.Background()
	var nextShift int64
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		// One shift is open at a time: the first closes before the next opens.
		if _, err := q.CloseShift(ctx, gen.CloseShiftParams{ClosedBy: 1, ClosedAt: sql.NullTime{Time: closed, Valid: true}, ID: shiftID}); err != nil {
			return err
		}
		var err error
		nextShift, err = q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "sh_CLOSE0002", VenueID: venueID, TradingDay: "2026-10-22", OpenedBy: 1, OpenedAt: closed.Add(10 * time.Hour)})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	third := addReport(t, d, venueID, nextShift, 3, fullDay(), 0)

	all, err := Since(ctx, d.Queries(), venueID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].ID != first || all[1].ID != second || all[2].ID != third {
		t.Fatalf("from the beginning: %+v", all)
	}
	if all[1].Corrects != 1 || all[1].Reason != "a tender voided late" {
		t.Errorf("the correction reads %+v", all[1])
	}

	after, err := Since(ctx, d.Queries(), venueID, first, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 || after[0].ID != second {
		t.Errorf("after the first: %+v", after)
	}
	one, err := Since(ctx, d.Queries(), venueID, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].ID != first {
		t.Errorf("limited to one: %+v", one)
	}
	none, err := Since(ctx, d.Queries(), venueID, third, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("past the last: %+v", none)
	}
}

func TestAReportThatDoesNotNameTheCloserReadsAsNobody(t *testing.T) {
	d, venueID, shiftID := newDB(t)
	doc := fullDay()
	doc.Names = nil
	addReport(t, d, venueID, shiftID, 1, doc, 0)
	got, err := Since(context.Background(), d.Queries(), venueID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ClosedBy != "" {
		t.Errorf("ClosedBy = %q, want nobody", got[0].ClosedBy)
	}
}

func TestABodyThatCannotBeReadIsAnErrorNamingTheReport(t *testing.T) {
	d, venueID, shiftID := newDB(t)
	ctx := context.Background()
	err := d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateZReport(ctx, gen.CreateZReportParams{
			VenueID: venueID, Number: 7, ShiftID: shiftID, Body: `{"report": "not a report"}`, GeneratedBy: 1, GeneratedAt: closed,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Since(ctx, d.Queries(), venueID, 0, 0); err == nil {
		t.Fatal("Since read a body that is not a report")
	} else if want := "Z-report 7"; !contains(err.Error(), want) {
		t.Errorf("error %q does not name %q", err, want)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
