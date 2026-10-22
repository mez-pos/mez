package watched

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
)

var start = time.Date(2026, 10, 22, 4, 30, 0, 0, time.UTC)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// The venue: Priya, a manager (shift.manage), and Asha, a cashier, and one
// table, T4.
type venue struct {
	d     *db.DB
	id    int64
	table int64
	priya int64
	asha  int64
}

func newVenue(t *testing.T) *venue {
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
	v := &venue{d: d}
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		ven, err := q.CreateVenue(ctx, gen.CreateVenueParams{
			PublicID: "ven_WATCH0001", Name: "Cafe", Currency: "INR", Timezone: "Asia/Kolkata",
			FinancialYearStartMonth: 4, TaxProfile: "none", CashRoundingStepMinor: 1, CreatedAt: start, UpdatedAt: start,
		})
		if err != nil {
			return err
		}
		v.id = ven.ID
		manager, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.id, Name: "manager", Permissions: `["shift.manage","line.void"]`, CreatedAt: start})
		if err != nil {
			return err
		}
		cashier, err := q.CreateRole(ctx, gen.CreateRoleParams{VenueID: v.id, Name: "cashier", Permissions: `["order.open"]`, CreatedAt: start})
		if err != nil {
			return err
		}
		for _, m := range []struct {
			into *int64
			pub  string
			name string
			role int64
		}{{&v.priya, "stf_PRIYA0001", "Priya", manager.ID}, {&v.asha, "stf_ASHA00001", "Asha", cashier.ID}} {
			s, err := q.CreateStaff(ctx, gen.CreateStaffParams{PublicID: m.pub, VenueID: v.id, RoleID: m.role, Name: m.name, PinHash: "x", Active: true, CreatedAt: start, UpdatedAt: start})
			if err != nil {
				return err
			}
			*m.into = s.ID
		}
		area, err := q.CreateArea(ctx, gen.CreateAreaParams{VenueID: v.id, Name: "Inside", CreatedAt: start, UpdatedAt: start})
		if err != nil {
			return err
		}
		v.table, err = q.CreateTable(ctx, gen.CreateTableParams{VenueID: v.id, AreaID: area, Name: "T4", Seats: 4, Shape: "square", X: 1, Y: 1, CreatedAt: start, UpdatedAt: start})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type raw struct {
	typ     order.Type
	payload any
	actor   int64
}

// order writes an order and its events, as the till would have, from at,
// and returns the ids of its events in the venue's stream.
func (v *venue) order(t *testing.T, publicID string, at time.Time, events []raw) []int64 {
	t.Helper()
	ctx := context.Background()
	err := v.d.WriteTx(ctx, func(q *gen.Queries) error {
		orderID, err := q.CreateOrder(ctx, gen.CreateOrderParams{
			PublicID: publicID, VenueID: v.id, Channel: "counter", TableID: v.table, Status: "open", LastSeq: int64(len(events)),
			OpenedAt: at, UpdatedAt: at,
		})
		if err != nil {
			return err
		}
		for i, e := range events {
			body, err := json.Marshal(e.payload)
			if err != nil {
				return err
			}
			if err := q.AppendOrderEvent(ctx, gen.AppendOrderEventParams{
				OrderID: orderID, Seq: int64(i + 1), Type: string(e.typ), Payload: string(body), ActorID: e.actor, At: at.Add(time.Duration(i) * time.Minute),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := v.d.VenueEventsAfter(ctx, v.id, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range rows {
		if r.OrderPublicID == publicID {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func (v *venue) line(id int64, name string, qty, price int64) raw {
	return raw{order.TypeLineAdded, order.LineAdded{ItemID: id, Name: name, Qty: qty, UnitPrice: inr(price), Modifiers: []order.Modifier{}, Course: 1, Station: "bar"}, v.asha}
}

func (v *venue) opened() raw {
	return raw{order.TypeOrderOpened, order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", Service: order.ServiceDineIn, TableID: v.table, Covers: 2, OpenedBy: v.asha}, v.asha}
}

// aDay is two orders: one with a void, a comp and a discount, and one paid
// in full, then voided, then refunded.
func (v *venue) aDay(t *testing.T) (first, second []int64) {
	t.Helper()
	first = v.order(t, "ord_DAY00001", start, []raw{
		v.opened(),
		v.line(1, "Flat White", 2, 26000),
		v.line(2, "Croissant", 1, 18000),
		{order.TypeLineVoided, order.LineVoided{LineID: 2, ReasonCode: "wrong_item", AuthorisedBy: v.priya}, v.asha},
		{order.TypeLineComped, order.LineComped{LineID: 3, ReasonCode: "spill", AuthorisedBy: v.priya}, v.priya},
		v.line(3, "Latte", 1, 30000),
		{order.TypeDiscountApplied, order.DiscountApplied{Scope: order.ScopeOrder, Kind: order.DiscountPercent, BasisPoints: 1000, Reason: "regular"}, v.asha},
	})
	second = v.order(t, "ord_DAY00002", start.Add(time.Hour), []raw{
		v.opened(),
		v.line(4, "Chai", 1, 4000),
		{order.TypePaymentTendered, order.PaymentTendered{Method: order.PaymentCash, Amount: inr(4000), Tip: inr(0), TenderedBy: v.asha}, v.asha},
		{order.TypeLineVoided, order.LineVoided{LineID: 2, ReasonCode: "not_made", AuthorisedBy: v.priya}, v.asha},
		{order.TypeRefundIssued, order.RefundIssued{Amount: inr(4000), AgainstPaymentID: 3, Reason: "not made", AuthorisedBy: v.priya}, v.asha},
	})
	return first, second
}

func TestSinceWordsEachWatchedEventAsTheProjectionValuesIt(t *testing.T) {
	v := newVenue(t)
	a, b := v.aDay(t)
	got, last, err := Since(context.Background(), v.d, v.id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if last != b[len(b)-1] {
		t.Errorf("last = %d, want the last event read, %d", last, b[len(b)-1])
	}
	asha, priya := Person{Name: "Asha"}, Person{Name: "Priya", Manager: true}
	later := start.Add(time.Hour)
	want := []Event{
		{ID: a[3], Kind: Void, At: start.Add(3 * time.Minute), OrderRef: "ord_DAY00001", Table: "T4", Item: "Flat White", Qty: 2, Amount: 52000, Currency: "INR", Reason: "wrong_item", By: asha, AuthorisedBy: priya},
		{ID: a[4], Kind: Comp, At: start.Add(4 * time.Minute), OrderRef: "ord_DAY00001", Table: "T4", Item: "Croissant", Qty: 1, Amount: 18000, Currency: "INR", Reason: "spill", By: priya, AuthorisedBy: priya},
		{ID: a[6], Kind: Discount, At: start.Add(6 * time.Minute), OrderRef: "ord_DAY00001", Table: "T4", Amount: 3000, Currency: "INR", Reason: "regular", By: asha},
		{ID: b[3], Kind: Void, At: later.Add(3 * time.Minute), OrderRef: "ord_DAY00002", Table: "T4", Item: "Chai", Qty: 1, Amount: 4000, Currency: "INR", Reason: "not_made", By: asha, AuthorisedBy: priya},
		{ID: b[4], Kind: Refund, At: later.Add(4 * time.Minute), OrderRef: "ord_DAY00002", Table: "T4", Amount: 4000, Currency: "INR", Reason: "not made", By: asha, AuthorisedBy: priya},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}

func TestSinceIsACursorOverEveryEvent(t *testing.T) {
	v := newVenue(t)
	a, b := v.aDay(t)
	ctx := context.Background()

	// The first two events are not watched, and the cursor still moves past them.
	got, last, err := Since(ctx, v.d, v.id, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || last != a[1] {
		t.Errorf("limited to two: %d events, last %d, want none and %d", len(got), last, a[1])
	}
	got, last, err = Since(ctx, v.d, v.id, a[3], 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].Kind != Comp || last != b[4] {
		t.Errorf("after the first void: %d events, first %s, last %d", len(got), got[0].Kind, last)
	}
	got, last, err = Since(ctx, v.d, v.id, b[4], 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || last != 0 {
		t.Errorf("at the end: %d events, last %d, want none and 0", len(got), last)
	}
}

func TestAPersonTheTableDoesNotHaveIsNobody(t *testing.T) {
	v := newVenue(t)
	v.order(t, "ord_GUEST0001", start, []raw{
		{order.TypeOrderOpened, order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", Service: order.ServiceDineIn, TableID: v.table, Covers: 1}, 0},
		{order.TypeLineAdded, order.LineAdded{ItemID: 1, Name: "Chai", Qty: 1, UnitPrice: inr(4000), Modifiers: []order.Modifier{}, Course: 1, Station: "bar"}, 0},
		{order.TypeLineVoided, order.LineVoided{LineID: 2, ReasonCode: "changed_mind", AuthorisedBy: 99}, 0},
	})
	got, _, err := Since(context.Background(), v.d, v.id, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].By != (Person{}) || got[0].AuthorisedBy != (Person{}) || got[0].Amount != 4000 {
		t.Errorf("got %+v", got)
	}
}
