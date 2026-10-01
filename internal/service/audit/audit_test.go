package audit

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// 12:00 in Kolkata on 1 October: trading day 2026-10-01.
var now = time.Date(2026, 10, 1, 6, 30, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

type fixture struct {
	s      *Service
	orders *svcorder.Service
	asha   int64 // a cashier
	priya  int64 // the owner, who authorises
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
	var f fixture
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_AUDIT001", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
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
		_, err = q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_AUDIT001", VenueID: 1, TradingDay: "2026-10-01", OpenedBy: f.priya, OpenedAt: now})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.orders, err = svcorder.New(svcorder.Options{DB: d, Clock: clock{}}); err != nil {
		t.Fatal(err)
	}
	if f.s, err = New(d, f.orders); err != nil {
		t.Fatal(err)
	}
	return f
}

func must(t *testing.T) func(svcorder.Result, error) svcorder.Result {
	return func(r svcorder.Result, err error) svcorder.Result {
		t.Helper()
		if err != nil {
			t.Fatalf("command: %v", err)
		}
		return r
	}
}

func line(name string, qty, price int64) svcorder.AddLine {
	return svcorder.AddLine{ItemID: 1, Name: name, Qty: qty, UnitPrice: inr(price), Station: "kitchen"}
}

// a day's corrections. The first order: a void Priya authorised, a comp, a
// line discount and a whole-bill discount, a voided payment, and a reopen.
// The second, part paid, has its only line voided, so it owes the guest,
// who is refunded part of it.
func (f fixture) aDay(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	m := svcorder.Meta{VenueID: 1, ActorID: f.asha, DeviceID: 1}
	id := must(t)(f.orders.Open(ctx, m, svcorder.OpenOrder{Channel: order.ChannelCounter})).OrderID
	must(t)(f.orders.AddLine(ctx, m, id, line("Dosa", 2, 12000)))  // line 2
	must(t)(f.orders.AddLine(ctx, m, id, line("Chai", 1, 3000)))   // line 3
	must(t)(f.orders.AddLine(ctx, m, id, line("Coffee", 3, 4000))) // line 4
	must(t)(f.orders.AddLine(ctx, m, id, line("Vada", 1, 5000)))   // line 5
	must(t)(f.orders.VoidLine(ctx, m, id, svcorder.VoidLine{LineID: 3, ReasonCode: "wrong_item", AuthorisedBy: f.priya}))
	must(t)(f.orders.CompLine(ctx, m, id, svcorder.CompLine{LineID: 5, ReasonCode: "regular", AuthorisedBy: f.priya}))
	must(t)(f.orders.ApplyDiscount(ctx, m, id, svcorder.ApplyDiscount{
		Scope: order.ScopeLine, LineID: 4, Kind: order.DiscountPercent, BasisPoints: 5000, Reason: "happy hour", AuthorisedBy: f.priya,
	}))
	res := must(t)(f.orders.ApplyDiscount(ctx, m, id, svcorder.ApplyDiscount{
		Scope: order.ScopeOrder, Kind: order.DiscountAmount, Amount: ptr(inr(1000)), Reason: "loyalty", AuthorisedBy: f.asha,
	}))
	due := res.State.Totals.BalanceDue
	first := must(t)(f.orders.TenderPayment(ctx, m, id, svcorder.TenderPayment{Method: order.PaymentCard, Amount: inr(1000)}))
	cardID := first.Events[0].Seq
	must(t)(f.orders.VoidPayment(ctx, m, id, svcorder.VoidPayment{PaymentID: cardID, Reason: "card declined", AuthorisedBy: f.priya}))
	must(t)(f.orders.TenderPayment(ctx, m, id, svcorder.TenderPayment{Method: order.PaymentCash, Amount: due}))
	must(t)(f.orders.Close(ctx, m, id))
	must(t)(f.orders.Reopen(ctx, m, id, svcorder.Reopen{Reason: "wrong tender", AuthorisedBy: f.priya}))

	second := must(t)(f.orders.Open(ctx, m, svcorder.OpenOrder{Channel: order.ChannelCounter})).OrderID
	must(t)(f.orders.AddLine(ctx, m, second, line("Thali", 1, 12000))) // line 2
	part := must(t)(f.orders.TenderPayment(ctx, m, second, svcorder.TenderPayment{Method: order.PaymentCash, Amount: inr(10000)}))
	must(t)(f.orders.VoidLine(ctx, m, second, svcorder.VoidLine{LineID: 2, ReasonCode: "out_of_stock", AuthorisedBy: f.priya}))
	must(t)(f.orders.Refund(ctx, m, second, svcorder.Refund{Amount: inr(5000), AgainstPaymentID: part.Events[0].Seq, Reason: "cold food", AuthorisedBy: f.priya}))
}

func ptr[T any](v T) *T { return &v }

func day(from, to string) Filter { return Filter{From: from, To: to} }

func TestTheTrail(t *testing.T) {
	f := newFixture(t)
	f.aDay(t)
	ctx := context.Background()

	page, err := f.s.Trail(ctx, 1, day("2026-10-01", "2026-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 8 || page.NextBefore != 0 {
		t.Fatalf("%d entries, next %d", len(page.Entries), page.NextBefore)
	}
	byKind := map[Kind][]Entry{}
	for _, e := range page.Entries {
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	want := map[Kind]int64{Void: 2, Comp: 1, Discount: 2, PaymentVoid: 1, Reopen: 1, Refund: 1}
	for k, n := range want {
		if int64(len(byKind[k])) != n || page.Counts[k] != n {
			t.Errorf("%s: %d entries, counted %d; want %d", k, len(byKind[k]), page.Counts[k], n)
		}
	}
	if page.Entries[0].Kind != Refund {
		t.Errorf("newest first: the first is %s", page.Entries[0].Kind)
	}

	v := byKind[Void][1] // the older: newest first
	if v.Item != "Chai" || v.Qty != 1 || v.Amount == nil || *v.Amount != inr(3000) || v.Reason != "wrong_item" ||
		v.By == nil || v.By.Name != "Asha" || v.AuthorisedBy == nil || v.AuthorisedBy.Name != "Priya" || v.OrderID == "" {
		t.Errorf("void = %+v", v)
	}
	if c := byKind[Comp][0]; c.Item != "Vada" || *c.Amount != inr(5000) {
		t.Errorf("comp = %+v", c)
	}
	for _, d := range byKind[Discount] {
		switch d.Reason {
		case "happy hour":
			if d.Item != "Coffee" || *d.Amount != inr(6000) || d.AuthorisedBy.Name != "Priya" {
				t.Errorf("line discount = %+v; half of 3 coffees at 40", d)
			}
		case "loyalty":
			if d.Item != "" || *d.Amount != inr(1000) || d.AuthorisedBy.Name != "Asha" {
				t.Errorf("bill discount = %+v", d)
			}
		}
	}
	if p := byKind[PaymentVoid][0]; *p.Amount != inr(1000) || p.Reason != "card declined" {
		t.Errorf("payment void = %+v", p)
	}
	if r := byKind[Refund][0]; *r.Amount != inr(5000) || r.Reason != "cold food" || r.AuthorisedBy.Name != "Priya" {
		t.Errorf("refund = %+v", r)
	}
	if r := byKind[Reopen][0]; r.Amount != nil || r.Reason != "wrong tender" {
		t.Errorf("reopen = %+v", r)
	}
}

func TestFiltering(t *testing.T) {
	f := newFixture(t)
	f.aDay(t)
	ctx := context.Background()
	count := func(fl Filter) int {
		t.Helper()
		page, err := f.s.Trail(ctx, 1, fl)
		if err != nil {
			t.Fatal(err)
		}
		return len(page.Entries)
	}
	all := day("2026-10-01", "2026-10-01")
	tests := []struct {
		name   string
		filter func(Filter) Filter
		want   int
	}{
		{"everything", func(fl Filter) Filter { return fl }, 8},
		{"only voids", func(fl Filter) Filter { fl.Kind = Void; return fl }, 2},
		{"only discounts", func(fl Filter) Filter { fl.Kind = Discount; return fl }, 2},
		// Asha did all eight; Priya authorised seven, and did none.
		{"what Asha did or authorised", func(fl Filter) Filter { fl.StaffID = "stf_ASHA0001"; return fl }, 8},
		{"what Priya authorised", func(fl Filter) Filter { fl.StaffID = "stf_PRIYA001"; return fl }, 7},
		{"the day before", func(Filter) Filter { return day("2026-09-30", "2026-09-30") }, 0},
		{"a week either side", func(Filter) Filter { return day("2026-09-24", "2026-10-08") }, 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := count(tt.filter(all)); got != tt.want {
				t.Errorf("%d entries, want %d", got, tt.want)
			}
		})
	}

	t.Run("pages", func(t *testing.T) {
		seen, before, pages := 0, int64(0), 0
		for {
			fl := all
			fl.Limit, fl.Before = 3, before
			page, err := f.s.Trail(ctx, 1, fl)
			if err != nil {
				t.Fatal(err)
			}
			pages++
			seen += len(page.Entries)
			if page.Counts[Discount] != 2 {
				t.Error("the counts cover every page, not this one")
			}
			if page.NextBefore == 0 {
				break
			}
			before = page.NextBefore
		}
		if seen != 8 || pages != 3 {
			t.Errorf("%d entries over %d pages, want 8 over 3", seen, pages)
		}
	})

	refusals := []struct {
		name string
		f    Filter
		is   error
	}{
		{"someone not here", Filter{From: "2026-10-01", To: "2026-10-01", StaffID: "stf_NOBODY01"}, ErrStaffNotFound},
		{"a day that is not one", day("yesterday", "2026-10-01"), ErrBadFilter},
		{"backwards", day("2026-10-02", "2026-10-01"), ErrBadFilter},
		{"over a year", day("2024-01-01", "2026-10-01"), ErrBadFilter},
		{"a kind we do not have", Filter{From: "2026-10-01", To: "2026-10-01", Kind: "theft"}, ErrBadFilter},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.s.Trail(ctx, 1, tt.f); !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want %v", err, tt.is)
			}
		})
	}
}

func TestNewNeedsBoth(t *testing.T) {
	if _, err := New(nil, nil); err == nil {
		t.Error("New with nothing must fail")
	}
}

func TestAnEmptyDayCountsEveryKindAsNone(t *testing.T) {
	f := newFixture(t)
	page, err := f.s.Trail(context.Background(), 1, day("2026-10-01", "2026-10-01"))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 || len(page.Counts) != len(Kinds()) {
		t.Fatalf("page = %+v", page)
	}
	for _, k := range Kinds() {
		if n, ok := page.Counts[k]; !ok || n != 0 {
			t.Errorf("%s = %d (present %v), want 0", k, n, ok)
		}
	}
}
