package floor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/table"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// moving is a clock both services share, moved on by the test, so an
// order's closing and a table's clearing fall in the order they happen.
type moving struct {
	mu sync.Mutex
	at time.Time
}

func (m *moving) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.at
}

func (m *moving) tick() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.at = m.at.Add(time.Minute)
}

// liveFloor is the floor with a real order service over one database, and
// the venues its OnChange was told about.
type liveFloor struct {
	floor   *Service
	orders  *svcorder.Service
	clock   *moving
	changed []int64
}

func newLive(t *testing.T) *liveFloor {
	t.Helper()
	d := openDB(t)
	w := &liveFloor{clock: &moving{at: now}}
	orders, err := svcorder.New(svcorder.Options{DB: d, Clock: w.clock})
	if err != nil {
		t.Fatal(err)
	}
	w.orders = orders
	w.floor, err = New(Options{DB: d, Orders: orders, Clock: w.clock, OnChange: func(v int64) { w.changed = append(w.changed, v) }})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

var staff = svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}

// tile is one table's tile on the live floor.
func (w *liveFloor) tile(t *testing.T, id int64) LiveTable {
	t.Helper()
	live := must(w.floor.Live(context.Background(), 1))
	for _, lt := range live.Tables {
		if lt.Table.ID == id {
			return lt
		}
	}
	t.Fatalf("table %d is not on the live floor", id)
	return LiveTable{}
}

func (w *liveFloor) want(t *testing.T, id int64, status table.Status) LiveTable {
	t.Helper()
	lt := w.tile(t, id)
	if lt.View.Status != status {
		t.Fatalf("table %s is %s, want %s", lt.Table.Name, lt.View.Status, status)
	}
	return lt
}

// TestATableTurn: seated, ordered, settled, dirty, cleared, each read from
// the orders as they happen.
func TestATableTurn(t *testing.T) {
	w := newLive(t)
	ctx := context.Background()
	inside := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	t4 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T4", Seats: 4, Shape: Round}))
	t5 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T5", Seats: 2, Shape: Square, X: 1}))

	if lt := w.want(t, t4, table.StatusFree); lt.View.BalanceValid || len(lt.Orders) != 0 {
		t.Errorf("a free table: %+v", lt)
	}

	w.clock.tick()
	opened := must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t4, Covers: 3}))
	lt := w.want(t, t4, table.StatusSeated)
	if lt.View.Covers != 3 || !lt.View.Since.Equal(w.clock.Now()) || len(lt.Orders) != 1 || lt.Orders[0] != opened.PublicID {
		t.Errorf("seated: %+v", lt)
	}
	w.want(t, t5, table.StatusFree)

	w.clock.tick()
	rupees := money.Amount{Minor: 45000, Currency: "INR"}
	must(w.orders.AddLine(ctx, staff, opened.OrderID, svcorder.AddLine{ItemID: 1, Name: "Thali", Qty: 1, UnitPrice: rupees, Station: "kitchen"}))
	if lt := w.want(t, t4, table.StatusOrdered); !lt.View.BalanceValid || lt.View.Balance != rupees {
		t.Errorf("ordered: balance %+v", lt.View.Balance)
	}

	w.clock.tick()
	paid := must(w.orders.TenderPayment(ctx, staff, opened.OrderID, svcorder.TenderPayment{Method: order.PaymentCash, Amount: rupees}))
	if paid.State.Status != order.StatusClosed {
		must(w.orders.Close(ctx, staff, opened.OrderID))
	}
	closedAt := w.clock.Now()
	if lt := w.want(t, t4, table.StatusDirty); !lt.View.Since.Equal(closedAt) || lt.View.BalanceValid {
		t.Errorf("dirty: %+v", lt.View)
	}

	w.changed = nil
	w.clock.tick()
	if err := w.floor.Clear(ctx, 1, t4); err != nil {
		t.Fatal(err)
	}
	w.want(t, t4, table.StatusFree)
	if len(w.changed) != 1 || w.changed[0] != 1 {
		t.Errorf("OnChange told %v", w.changed)
	}
	// Cleared twice by two waiters: no harm.
	if err := w.floor.Clear(ctx, 1, t4); err != nil {
		t.Errorf("cleared again: %v", err)
	}

	// Seated again, it cannot be cleared from under the guests.
	w.clock.tick()
	must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t4, Covers: 2}))
	if err := w.floor.Clear(ctx, 1, t4); !errors.Is(err, table.ErrOccupied) {
		t.Errorf("cleared an occupied table: %v", err)
	}
	w.want(t, t4, table.StatusSeated)
}

// TestTheLiveFloorShowsOnlyWhatIsInService: retired tables and areas stay
// off it, and orders with no table or at another venue sit nowhere.
func TestTheLiveFloorShowsOnlyWhatIsInService(t *testing.T) {
	w := newLive(t)
	ctx := context.Background()
	inside := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	terrace := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Terrace"}))
	t1 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T1", Seats: 2, Shape: Square}))
	gone := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T2", Seats: 2, Shape: Square, X: 1}))
	if err := w.floor.SetTableActive(ctx, 1, gone, false); err != nil {
		t.Fatal(err)
	}
	if err := w.floor.SetAreaActive(ctx, 1, terrace, false); err != nil {
		t.Fatal(err)
	}
	must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, Covers: 1}))

	live := must(w.floor.Live(ctx, 1))
	if len(live.Areas) != 1 || live.Areas[0].ID != inside {
		t.Errorf("areas = %+v", live.Areas)
	}
	if len(live.Tables) != 1 || live.Tables[0].Table.ID != t1 || live.Tables[0].View.Status != table.StatusFree {
		t.Errorf("tables = %+v", live.Tables)
	}
	if err := w.floor.Clear(ctx, 1, gone); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("cleared a retired table: %v", err)
	}
	if err := w.floor.Clear(ctx, 2, t1); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("cleared another venue's table: %v", err)
	}
}

func TestLiveNeedsTheOrders(t *testing.T) {
	if _, err := newService(t).Live(context.Background(), 1); err == nil {
		t.Error("read the live floor without orders")
	}
}

// TestEveryPlanChangeMovesTheVersion: the stream learns of plan edits too.
func TestEveryPlanChangeMovesTheVersion(t *testing.T) {
	w := newLive(t)
	ctx := context.Background()
	area := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	id := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: area, Name: "T1", Seats: 2, Shape: Square}))
	steps := []error{
		w.floor.RenameArea(ctx, 1, area, AreaSettings{Name: "Hall"}),
		w.floor.UpdateTable(ctx, 1, id, TableSettings{AreaID: area, Name: "T1", Seats: 4, Shape: Square}),
		w.floor.SetTableActive(ctx, 1, id, false),
	}
	for i, err := range steps {
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	// A refused change moves nothing.
	if _, err := w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Hall"}); err == nil {
		t.Fatal("a second Hall")
	}
	if len(w.changed) != 5 {
		t.Errorf("OnChange told %d times, want 5", len(w.changed))
	}
	if v := must(w.floor.db.FloorVersion(ctx, 1)); v != 5 {
		t.Errorf("floor version = %d, want 5", v)
	}
}

// TestGuestsLeave: a party that sat and ordered nothing goes, and the table
// is free again rather than dirty; one with something on the ticket cannot.
func TestGuestsLeave(t *testing.T) {
	w := newLive(t)
	ctx := context.Background()
	inside := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	t1 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T1", Seats: 2, Shape: Square}))
	t2 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T2", Seats: 2, Shape: Square, X: 1}))
	meta := staff
	meta.IdempotencyKey = "guests-left-0001"

	// Two empty tickets at T1: both close, with one key between them.
	w.clock.tick()
	first := must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t1, Covers: 2}))
	must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t1, Covers: 1}))
	w.clock.tick()
	if err := w.floor.Release(ctx, meta, t1); err != nil {
		t.Fatal(err)
	}
	if lt := w.want(t, t1, table.StatusFree); len(lt.Orders) != 0 {
		t.Errorf("orders still at T1: %v", lt.Orders)
	}
	if res := must(w.orders.Order(ctx, 1, first.OrderID)); res.State.Status != order.StatusClosed || len(res.State.Invoices) != 0 {
		t.Errorf("the empty ticket: %s, %d invoices", res.State.Status, len(res.State.Invoices))
	}
	// Retried: nothing open, the table clear. No harm.
	if err := w.floor.Release(ctx, meta, t1); err != nil {
		t.Errorf("retried: %v", err)
	}

	// Something on the ticket: refused, and the table stays as it was.
	opened := must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t2, Covers: 2}))
	must(w.orders.AddLine(ctx, staff, opened.OrderID, svcorder.AddLine{ItemID: 1, Name: "Tea", Qty: 1, UnitPrice: money.Amount{Minor: 3000, Currency: "INR"}, Station: "bar"}))
	if err := w.floor.Release(ctx, staff, t2); !errors.Is(err, table.ErrNotEmpty) {
		t.Errorf("released T2 with tea on it: %v", err)
	}
	w.want(t, t2, table.StatusOrdered)

	if err := w.floor.Release(ctx, staff, 999); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("released no table: %v", err)
	}
	if err := newService(t).Release(ctx, staff, t1); err == nil {
		t.Error("released without the orders")
	}
}

// TestAJoinedParty: the party shows at every table it sits at, counted and
// owing at its own; its tables all go dirty when the bill closes.
func TestAJoinedParty(t *testing.T) {
	w := newLive(t)
	ctx := context.Background()
	inside := must(w.floor.CreateArea(ctx, 1, AreaSettings{Name: "Inside"}))
	t1 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T1", Seats: 4, Shape: Square}))
	t2 := must(w.floor.CreateTable(ctx, 1, TableSettings{AreaID: inside, Name: "T2", Seats: 4, Shape: Square, X: 1}))

	w.clock.tick()
	party := must(w.orders.Open(ctx, staff, svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: t1, Covers: 7}))
	must(w.orders.JoinTables(ctx, staff, party.OrderID, svcorder.JoinTables{Tables: []int64{t2}}))
	own, joined := w.want(t, t1, table.StatusSeated), w.want(t, t2, table.StatusSeated)
	if own.JoinedTo != 0 || own.View.Covers != 7 || !own.View.BalanceValid {
		t.Errorf("T1 = %+v", own)
	}
	if joined.JoinedTo != t1 || joined.View.Covers != 0 || joined.View.BalanceValid || len(joined.Orders) != 1 || joined.Orders[0] != party.PublicID {
		t.Errorf("T2 = %+v", joined)
	}
	// Guests left from the joined table: the whole empty party goes.
	w.clock.tick()
	if err := w.floor.Release(ctx, staff, t2); err != nil {
		t.Fatal(err)
	}
	// Both tables are free: the party got up from both.
	w.want(t, t2, table.StatusFree)
	w.want(t, t1, table.StatusFree)
}
