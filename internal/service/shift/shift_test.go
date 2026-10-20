package shift

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/fsm"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/shift"
)

// 23:30 IST on 27 September: an evening shift.
var start = time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)

// clock is a settable test clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// openDB is a fresh database: venues 1 and 2, both in Asia/Kolkata and INR.
func openDB(t *testing.T) *db.DB {
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
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		for _, v := range []string{"ven_SHIFT001", "ven_SHIFT002"} {
			if _, err := q.CreateVenue(ctx, gen.CreateVenueParams{
				PublicID: v, Name: "Bar", Currency: "INR", Timezone: "Asia/Kolkata",
				FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: start, UpdatedAt: start,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type fixture struct {
	svc     *Service
	db      *db.DB
	clock   *clock
	changed []int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{db: openDB(t), clock: &clock{now: start}}
	svc, err := New(Options{DB: f.db, Clock: f.clock, OnChange: func(v int64) { f.changed = append(f.changed, v) }})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f
}

var (
	cashier = Meta{VenueID: 1, ActorID: 3, DeviceID: 7}
	ctx     = context.Background()
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func float(t *testing.T, minor int64) shift.Count {
	t.Helper()
	c, err := shift.LumpSum(shift.CountOpening, inr(minor))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func movement(t *testing.T, kind shift.MovementKind, minor int64, reason string) shift.Movement {
	t.Helper()
	m, err := shift.NewMovement(kind, inr(minor), reason, 2, "INR")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// countExact counts the drawer at exactly what is expected, so the shift
// can close with no reason.
func countExact(t *testing.T, f *fixture, shiftID int64) {
	t.Helper()
	sh := must(f.svc.ByID(ctx, 1, shiftID))
	c, err := shift.LumpSum(shift.CountClosing, sh.Expected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Count(ctx, cashier, shiftID, c); err != nil {
		t.Fatal(err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestNewRefusals(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("no database: want an error")
	}
}

func TestAShiftFromOpenToClose(t *testing.T) {
	f := newFixture(t)

	denominated := must(shift.NewCount(shift.CountOpening, "INR", []shift.Denomination{{Face: 50000, Count: 2}, {Face: 1000, Count: 10}}))
	opened, err := f.svc.Open(ctx, cashier, denominated)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Status != shift.StatusOpen || opened.OpenedBy != 3 || !opened.OpenedAt.Equal(start) {
		t.Errorf("opened = %+v", opened)
	}
	if opened.TradingDay != "2026-09-27" {
		t.Errorf("trading day = %s, want 2026-09-27", opened.TradingDay)
	}
	if opened.Float.Total != inr(110000) || len(opened.Float.Denominations) != 2 {
		t.Errorf("float = %+v, want 1100.00 in two denominations", opened.Float)
	}
	if opened.Expected != inr(110000) {
		t.Errorf("drawer = %s, want the float", opened.Expected)
	}

	current := must(f.svc.Current(ctx, 1))
	if current.ID != opened.ID {
		t.Errorf("current = %d, want %d", current.ID, opened.ID)
	}

	// Past midnight, on the same shift: still the 27th's trade.
	f.clock.set(start.Add(3 * time.Hour))
	must(f.svc.MoveCash(ctx, cashier, opened.ID, movement(t, shift.CashIn, 50000, "coins from the safe")))
	moved := must(f.svc.MoveCash(ctx, cashier, opened.ID, movement(t, shift.CashOut, 12050, "ice from the shop")))
	if moved.Expected != inr(147950) {
		t.Errorf("drawer = %s, want 1479.50", moved.Expected)
	}
	if moved.TradingDay != "2026-09-27" {
		t.Errorf("trading day moved to %s", moved.TradingDay)
	}

	f.clock.set(start.Add(4 * time.Hour))
	countExact(t, f, opened.ID)
	closed, err := f.svc.Close(ctx, Meta{VenueID: 1, ActorID: 2, DeviceID: 7}, opened.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != shift.StatusClosed || closed.ClosedBy != 2 || !closed.ClosedAt.Equal(start.Add(4*time.Hour)) {
		t.Errorf("closed = %+v", closed)
	}

	var types []EventType
	for _, e := range closed.Events {
		types = append(types, e.Type)
	}
	want := []EventType{EventOpened, EventCashIn, EventCashOut, EventClosed}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events = %v, want %v", types, want)
		}
	}
	out := closed.Events[2]
	if out.Amount != inr(12050) || out.Reason != "ice from the shop" || out.AuthorisedBy != 2 || out.ActorID != 3 || out.DeviceID != 7 {
		t.Errorf("cash out = %+v", out)
	}

	if _, err := f.svc.Current(ctx, 1); !errors.Is(err, ErrNoOpenShift) {
		t.Errorf("after close: err = %v, want ErrNoOpenShift", err)
	}
	if len(f.changed) != 5 {
		t.Errorf("told of %d changes, want 5", len(f.changed))
	}

	// The next shift opens on a new day.
	f.clock.set(start.Add(15 * time.Hour)) // 14:30 IST on the 28th
	next := must(f.svc.Open(ctx, cashier, float(t, 200000)))
	if next.TradingDay != "2026-09-28" || next.ID == opened.ID {
		t.Errorf("next = %+v", next)
	}
}

func TestOneOpenShiftPerVenue(t *testing.T) {
	f := newFixture(t)
	must(f.svc.Open(ctx, cashier, float(t, 100000)))

	if _, err := f.svc.Open(ctx, cashier, float(t, 100000)); !errors.Is(err, ErrShiftOpen) {
		t.Errorf("a second shift: err = %v, want ErrShiftOpen", err)
	}
	// Another venue has its own drawer.
	if _, err := f.svc.Open(ctx, Meta{VenueID: 2, ActorID: 3}, float(t, 0)); err != nil {
		t.Errorf("venue 2: %v", err)
	}
}

func TestTheDatabaseHoldsOneOpenShift(t *testing.T) {
	f := newFixture(t)
	must(f.svc.Open(ctx, cashier, float(t, 100000)))
	err := f.db.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_RACE0001", VenueID: 1, TradingDay: "2026-09-27", OpenedBy: 3, OpenedAt: start})
		return err
	})
	if !db.IsUniqueViolation(err) {
		t.Errorf("a second open row: err = %v, want a unique violation", err)
	}
}

func TestOpenRefusals(t *testing.T) {
	tests := []struct {
		name  string
		meta  Meta
		float shift.Count
		want  error
	}{
		{"no staff member", Meta{VenueID: 1}, shift.Count{Kind: shift.CountOpening, Total: inr(0)}, ErrInvalidCommand},
		{"no venue", Meta{ActorID: 3}, shift.Count{Kind: shift.CountOpening, Total: inr(0)}, ErrInvalidCommand},
		{"an unknown venue", Meta{VenueID: 9, ActorID: 3}, shift.Count{Kind: shift.CountOpening, Total: inr(0)}, ErrVenueConfig},
		{"a closing count", cashier, shift.Count{Kind: shift.CountClosing, Total: inr(0)}, ErrInvalidCommand},
		{"a negative float", cashier, shift.Count{Kind: shift.CountOpening, Total: inr(-1)}, ErrInvalidCommand},
		{"a dollar float", cashier, shift.Count{Kind: shift.CountOpening, Total: money.Amount{Minor: 100, Currency: "USD"}}, ErrInvalidCommand},
		{"notes that do not add up", cashier, shift.Count{
			Kind: shift.CountOpening, Denominations: []shift.Denomination{{Face: 50000, Count: 1}}, Total: inr(60000),
		}, ErrInvalidCommand},
		{"a key with a newline", Meta{VenueID: 1, ActorID: 3, IdempotencyKey: "a\nb"}, shift.Count{Kind: shift.CountOpening, Total: inr(0)}, ErrInvalidCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if _, err := f.svc.Open(ctx, tt.meta, tt.float); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if _, err := f.svc.Current(ctx, 1); !errors.Is(err, ErrNoOpenShift) {
				t.Errorf("a refused open left a shift behind: %v", err)
			}
		})
	}
}

func TestMoveCashRefusals(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	other := must(f.svc.Open(ctx, Meta{VenueID: 2, ActorID: 3}, float(t, 0)))

	tests := []struct {
		name    string
		shiftID int64
		m       shift.Movement
		want    error
	}{
		{"no reason", open.ID, shift.Movement{Kind: shift.CashOut, Amount: inr(100), AuthorisedBy: 2}, ErrInvalidCommand},
		{"no manager", open.ID, shift.Movement{Kind: shift.CashOut, Amount: inr(100), Reason: "milk"}, ErrInvalidCommand},
		{"nothing moved", open.ID, shift.Movement{Kind: shift.CashIn, Amount: inr(0), Reason: "milk", AuthorisedBy: 2}, ErrInvalidCommand},
		{"another venue's shift", other.ID, movement(t, shift.CashOut, 100, "milk"), ErrShiftNotFound},
		{"no such shift", 99, movement(t, shift.CashOut, 100, "milk"), ErrShiftNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.MoveCash(ctx, cashier, tt.shiftID, tt.m); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	if got := must(f.svc.ByID(ctx, 1, open.ID)); len(got.Events) != 1 {
		t.Errorf("refused movements were recorded: %+v", got.Events)
	}
}

func TestAClosedShiftTakesNothingMore(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	countExact(t, f, open.ID)
	must(f.svc.Close(ctx, cashier, open.ID))

	if _, err := f.svc.MoveCash(ctx, cashier, open.ID, movement(t, shift.CashOut, 100, "milk")); !errors.Is(err, ErrShiftClosed) {
		t.Errorf("cash out of a closed shift: err = %v, want ErrShiftClosed", err)
	}
	_, err := f.svc.Close(ctx, cashier, open.ID)
	if !errors.Is(err, ErrShiftClosed) || !errors.Is(err, fsm.ErrInvalidTransition) {
		t.Errorf("closing twice: err = %v, want ErrShiftClosed and ErrInvalidTransition", err)
	}
}

func TestRetriesAreRecordedOnce(t *testing.T) {
	f := newFixture(t)
	withKey := func(k string) Meta { m := cashier; m.IdempotencyKey = k; return m }

	first := must(f.svc.Open(ctx, withKey("open-1"), float(t, 100000)))
	again, err := f.svc.Open(ctx, withKey("open-1"), float(t, 100000))
	if err != nil || again.ID != first.ID {
		t.Fatalf("retried open = %d, %v; want shift %d", again.ID, err, first.ID)
	}
	if _, err := f.svc.Open(ctx, withKey("open-1"), float(t, 5)); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("the key for another float: err = %v, want ErrIdempotencyKeyReused", err)
	}

	payout := movement(t, shift.CashOut, 12050, "ice")
	must(f.svc.MoveCash(ctx, withKey("pay-1"), first.ID, payout))
	got := must(f.svc.MoveCash(ctx, withKey("pay-1"), first.ID, payout))
	if len(got.Events) != 2 || got.Expected != inr(87950) {
		t.Errorf("a retried payout counted twice: %d events, drawer %s", len(got.Events), got.Expected)
	}
	bigger := movement(t, shift.CashOut, 99999, "ice")
	if _, err := f.svc.MoveCash(ctx, withKey("pay-1"), first.ID, bigger); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("the key for another payout: err = %v, want ErrIdempotencyKeyReused", err)
	}

	countExact(t, f, first.ID)
	must(f.svc.Close(ctx, withKey("close-1"), first.ID))
	if _, err := f.svc.Close(ctx, withKey("close-1"), first.ID); err != nil {
		t.Errorf("a retried close: %v", err)
	}
	// A payout retried after close still gets its answer, not a refusal.
	if _, err := f.svc.MoveCash(ctx, withKey("pay-1"), first.ID, payout); err != nil {
		t.Errorf("a payout retried after close: %v", err)
	}
}

func TestTheRecordCannotBeRewritten(t *testing.T) {
	f := newFixture(t)
	open := must(f.svc.Open(ctx, cashier, float(t, 100000)))
	must(f.svc.MoveCash(ctx, cashier, open.ID, movement(t, shift.CashOut, 100, "milk")))

	tests := []struct {
		name string
		sql  string
	}{
		{"edit a movement", "UPDATE shift_events SET amount = 1"},
		{"delete a movement", "DELETE FROM shift_events"},
		{"edit the float", "UPDATE drawer_counts SET total = 1"},
		{"delete the float", "DELETE FROM drawer_counts"},
		{"move the trading day", "UPDATE shifts SET trading_day = '2026-09-28'"},
		{"delete the shift", "DELETE FROM shifts"},
		{"a movement with no reason", "INSERT INTO shift_events (shift_id, type, amount, reason, actor_id, authorised_by, at) VALUES (1, 'cash_out', 100, ' ', 3, 2, '2026-09-27 18:00:00+00:00')"},
		{"a movement nobody authorised", "INSERT INTO shift_events (shift_id, type, amount, reason, actor_id, at) VALUES (1, 'cash_out', 100, 'milk', 3, '2026-09-27 18:00:00+00:00')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := f.db.Tx(ctx, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, tt.sql)
				return err
			})
			if err == nil {
				t.Error("the database let it through")
			}
		})
	}

	countExact(t, f, open.ID)
	must(f.svc.Close(ctx, cashier, open.ID))
	err := f.db.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE shifts SET status = 'open', closed_at = NULL, closed_by = 0")
		return err
	})
	if err == nil {
		t.Error("a closed shift was reopened by hand")
	}
}
