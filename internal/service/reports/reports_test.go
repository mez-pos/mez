package reports

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/report"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

var kolkata, _ = time.LoadLocation("Asia/Kolkata")

func ist(day, hour, min int) time.Time {
	return time.Date(2026, 10, day, hour, min, 0, 0, kolkata).UTC()
}

// clock is the time the order service stamps events with; a test moves it.
type clock struct{ now *time.Time }

func (c clock) Now() time.Time { return *c.now }

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

type fixture struct {
	s      *Service
	orders *svcorder.Service
	now    *time.Time
	asha   int64
	priya  int64
}

func newFixture(t *testing.T) fixture {
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
	now := ist(1, 9, 0)
	f := fixture{now: &now}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_REPORT01", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		role, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: 1, Name: "owner", Permissions: `[]`, CreatedAt: now})
		if err != nil {
			return err
		}
		for _, p := range []struct {
			pub, name string
			into      *int64
		}{{"stf_ASHA0001", "Asha", &f.asha}, {"stf_PRIYA001", "Priya", &f.priya}} {
			s, err := q.CreateStaff(ctx, gen.CreateStaffParams{
				PublicID: p.pub, VenueID: 1, RoleID: role.ID, Name: p.name, PinHash: "x", Active: true, CreatedAt: now, UpdatedAt: now,
			})
			if err != nil {
				return err
			}
			*p.into = s.ID
		}
		// One shift over all three days: a range report places events by
		// when they happened, never by their shift.
		_, err = q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_REPORT01", VenueID: 1, TradingDay: "2026-10-01", OpenedBy: f.priya, OpenedAt: now})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.orders, err = svcorder.New(svcorder.Options{DB: d, Clock: clock{f.now}}); err != nil {
		t.Fatal(err)
	}
	if f.s, err = New(d); err != nil {
		t.Fatal(err)
	}
	return f
}

// sell rings up one bill at a time of day, pays it in cash and closes it,
// which issues its invoice.
func (f fixture) sell(t *testing.T, at time.Time, by int64, channel order.Channel, name string, price int64) {
	t.Helper()
	ctx := context.Background()
	*f.now = at
	m := svcorder.Meta{VenueID: 1, ActorID: by, DeviceID: 1}
	steps := func(r svcorder.Result, err error) svcorder.Result {
		t.Helper()
		if err != nil {
			t.Fatalf("selling %s: %v", name, err)
		}
		return r
	}
	id := steps(f.orders.Open(ctx, m, svcorder.OpenOrder{Channel: channel})).OrderID
	steps(f.orders.AddLine(ctx, m, id, svcorder.AddLine{ItemID: 1, Name: name, Qty: 1, UnitPrice: inr(price), Station: "kitchen"}))
	steps(f.orders.TenderPayment(ctx, m, id, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(price)}))
	steps(f.orders.Close(ctx, m, id))
}

// threeDays: two bills on Thursday 1 October, one of them at 01:30 on the
// Friday and so still Thursday's trade; none on Friday; one on Saturday.
func (f fixture) threeDays(t *testing.T) {
	f.sell(t, ist(1, 12, 0), f.asha, order.ChannelCounter, "Dosa", 12000)
	f.sell(t, ist(2, 1, 30), f.priya, order.ChannelQRTable, "Coffee", 4000)
	f.sell(t, ist(3, 19, 0), f.asha, order.ChannelCounter, "Thali", 20000)
}

func TestARange(t *testing.T) {
	f := newFixture(t)
	f.threeDays(t)
	got, err := f.s.Sales(context.Background(), 1, Query{From: "2026-10-01", To: "2026-10-03"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Every != report.Day {
		t.Errorf("every %q, want a day by default", got.Every)
	}
	if s := got.Totals.Sales; s.Bills != 3 || s.Total != inr(36000) {
		t.Errorf("sales: %d bills, %s; want 3, 360.00", s.Bills, s.Total)
	}
	wantDays := []int64{2, 0, 1}
	if len(got.Series) != len(wantDays) {
		t.Fatalf("series %+v", got.Series)
	}
	for i, p := range got.Series {
		if p.Sales.Bills != wantDays[i] {
			t.Errorf("%s: %d bills, want %d", p.Start, p.Sales.Bills, wantDays[i])
		}
	}
	staff := map[string]int64{}
	for _, g := range got.Totals.ByStaff {
		staff[g.Name] = g.Qty
	}
	if staff["Asha"] != 2 || staff["Priya"] != 1 || len(staff) != 2 {
		t.Errorf("by staff %+v, want Asha 2 and Priya 1, by name", got.Totals.ByStaff)
	}
	if got.Names[f.asha] != "Asha" {
		t.Errorf("names %+v", got.Names)
	}
	if len(got.Hours) != 3 {
		t.Errorf("hours %+v, want three cells", got.Hours)
	}
}

func TestOneDayLeavesTheOthersOut(t *testing.T) {
	f := newFixture(t)
	f.threeDays(t)
	for _, tt := range []struct {
		day   string
		bills int64
		total int64
	}{{"2026-10-01", 2, 16000}, {"2026-10-02", 0, 0}, {"2026-10-03", 1, 20000}, {"2026-09-30", 0, 0}} {
		got, err := f.s.Sales(context.Background(), 1, Query{From: tt.day, To: tt.day, Every: report.Week})
		if err != nil {
			t.Fatal(err)
		}
		if s := got.Totals.Sales; s.Bills != tt.bills || s.Total.Minor != tt.total {
			t.Errorf("%s: %d bills, %s; want %d, %d", tt.day, s.Bills, s.Total, tt.bills, tt.total)
		}
	}
}

func TestASheetOfTheRange(t *testing.T) {
	f := newFixture(t)
	f.threeDays(t)
	ctx := context.Background()
	q := Query{From: "2026-10-01", To: "2026-10-03"}

	// By name, as the report itself names staff, and in major units.
	staff, err := f.s.SalesTable(ctx, 1, q, report.SheetStaff)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]string{
		{"Asha", "2", "320.00", "0.00", "320.00", "0.00", "320.00"},
		{"Priya", "1", "40.00", "0.00", "40.00", "0.00", "40.00"},
	} {
		if !slices.ContainsFunc(staff.Rows, func(row []string) bool { return slices.Equal(row, want) }) {
			t.Errorf("no row %q in %q", want, staff.Rows)
		}
	}

	// The coffee at 01:30 on the 2nd is the 1st's trade.
	days, err := f.s.SalesTable(ctx, 1, q, report.SheetOverTime)
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	for _, row := range days.Rows {
		got = append(got, []string{row[0], row[2], row[8]})
	}
	want := [][]string{{"2026-10-01", "2", "160.00"}, {"2026-10-02", "0", "0.00"}, {"2026-10-03", "1", "200.00"}}
	if !slices.EqualFunc(got, want, func(a, b []string) bool { return slices.Equal(a, b) }) {
		t.Errorf("days %q, want %q", got, want)
	}

	if _, err := f.s.SalesTable(ctx, 1, q, "profit"); !errors.Is(err, ErrNoSheet) {
		t.Errorf("a sheet we do not have: err %v, want ErrNoSheet", err)
	}
	if _, err := f.s.SalesTable(ctx, 1, Query{From: "2026-10-03", To: "2026-10-01"}, report.SheetItems); !errors.Is(err, ErrBadRange) {
		t.Errorf("backwards: err %v, want ErrBadRange", err)
	}
}

func TestABadRange(t *testing.T) {
	f := newFixture(t)
	for _, q := range []Query{
		{From: "2026-10-03", To: "2026-10-01"},
		{From: "2026-10-01", To: "tomorrow"},
		{From: "2026-10-01", To: "2026-10-03", Every: "hour"},
		{From: "2025-01-01", To: "2026-10-03"},
	} {
		if _, err := f.s.Sales(context.Background(), 1, q); !errors.Is(err, ErrBadRange) {
			t.Errorf("%+v: err %v, want ErrBadRange", q, err)
		}
	}
}
